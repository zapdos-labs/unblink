package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	servicev1 "unb/server/gen/service/v1"
	"unb/server"
	"unb/database"
	"unb/server/webrtc"
)

// ServiceState tracks the state of a service
type ServiceState struct {
	ID        string
	Name      string
	URL       string
	NodeID    string
	Online    bool
	Extractor *webrtc.FrameExtractor
	Cancel    context.CancelFunc
}

// ServiceRegistry manages all services and their frame extractors
type ServiceRegistry struct {
	db            *database.Client
	storage  *webrtc.Storage
	frameInterval time.Duration
	srv           *server.Server

	mu       sync.RWMutex
	services map[string]*ServiceState      // serviceID -> ServiceState
	nodes    map[string]map[string]bool    // nodeID -> set of serviceIDs
	onlineNodes map[string]bool             // nodeID -> online status
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(db *database.Client, frameInterval time.Duration, framesDir string, srv *server.Server) *ServiceRegistry {
	storage := webrtc.NewStorage(framesDir)

	// Wire up callback to save frame metadata to database when frames are saved to disk
	storage.SetOnSaved(func(serviceID, frameID, framePath string, timestamp time.Time, fileSize int64, sequence int64) {
		metadata := &database.FrameMetadata{
			Sequence: sequence,
		}
		if err := db.SaveFrame(serviceID, framePath, timestamp, fileSize, metadata); err != nil {
			log.Printf("[ServiceRegistry] Failed to save frame metadata: %v", err)
		}
	})

	return &ServiceRegistry{
		db:            db,
		storage:       storage,
		frameInterval: frameInterval,
		srv:           srv,
		services:      make(map[string]*ServiceState),
		nodes:         make(map[string]map[string]bool),
		onlineNodes:   make(map[string]bool),
	}
}

// SetServer sets the server reference (needed after server is created)
func (r *ServiceRegistry) SetServer(srv *server.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.srv = srv
}

// AddService adds a service to the registry and starts extractor if node is online
func (r *ServiceRegistry) AddService(service *servicev1.Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if node is online
	nodeOnline := r.onlineNodes[service.NodeId]

	state := &ServiceState{
		ID:     service.Id,
		Name:   service.Name,
		URL:    service.Url,
		NodeID: service.NodeId,
		Online: nodeOnline,
	}

	r.services[service.Id] = state

	// Initialize node's service set
	if r.nodes[service.NodeId] == nil {
		r.nodes[service.NodeId] = make(map[string]bool)
	}
	r.nodes[service.NodeId][service.Id] = true

	// Start extractor if node is online
	if nodeOnline {
		r.startExtractorLocked(state)
	}

	log.Printf("[ServiceRegistry] Added service %s (node=%s, online=%v)", service.Id, service.NodeId, nodeOnline)
	return nil
}

// RemoveService removes a service from the registry and stops its extractor
func (r *ServiceRegistry) RemoveService(serviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, exists := r.services[serviceID]
	if !exists {
		return
	}

	// Stop extractor
	r.stopExtractorLocked(state)

	// Remove from node's service set
	if r.nodes[state.NodeID] != nil {
		delete(r.nodes[state.NodeID], serviceID)
		if len(r.nodes[state.NodeID]) == 0 {
			delete(r.nodes, state.NodeID)
		}
	}

	delete(r.services, serviceID)
	log.Printf("[ServiceRegistry] Removed service %s", serviceID)
}

// UpdateService updates a service in the registry
func (r *ServiceRegistry) UpdateService(service *servicev1.Service) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, exists := r.services[service.Id]
	if !exists {
		// Service doesn't exist, add it
		r.mu.Unlock()
		r.AddService(service)
		r.mu.Lock()
		return
	}

	// Stop old extractor
	r.stopExtractorLocked(state)

	// Update state
	state.Name = service.Name
	state.URL = service.Url

	// Handle node change
	if state.NodeID != service.NodeId {
		// Remove from old node
		if r.nodes[state.NodeID] != nil {
			delete(r.nodes[state.NodeID], service.Id)
			if len(r.nodes[state.NodeID]) == 0 {
				delete(r.nodes, state.NodeID)
			}
		}

		// Add to new node
		state.NodeID = service.NodeId
		if r.nodes[service.NodeId] == nil {
			r.nodes[service.NodeId] = make(map[string]bool)
		}
		r.nodes[service.NodeId][service.Id] = true
	}

	// Restart extractor if node is online
	state.Online = r.onlineNodes[state.NodeID]
	if state.Online {
		r.startExtractorLocked(state)
	}

	log.Printf("[ServiceRegistry] Updated service %s", service.Id)
}

// SetNodeOnline sets a node as online and starts extractors for all its services
func (r *ServiceRegistry) SetNodeOnline(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.onlineNodes[nodeID] = true

	serviceIDs := r.nodes[nodeID]
	if serviceIDs == nil {
		return
	}

	log.Printf("[ServiceRegistry] Node %s online, starting %d extractors", nodeID, len(serviceIDs))

	for serviceID := range serviceIDs {
		state := r.services[serviceID]
		if state != nil && !state.Online {
			state.Online = true
			r.startExtractorLocked(state)
		}
	}
}

// SetNodeOffline sets a node as offline and stops extractors for all its services
func (r *ServiceRegistry) SetNodeOffline(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.onlineNodes, nodeID)

	serviceIDs := r.nodes[nodeID]
	if serviceIDs == nil {
		return
	}

	log.Printf("[ServiceRegistry] Node %s offline, stopping %d extractors", nodeID, len(serviceIDs))

	for serviceID := range serviceIDs {
		state := r.services[serviceID]
		if state != nil && state.Online {
			state.Online = false
			r.stopExtractorLocked(state)
		}
	}
}

// startExtractorLocked starts the frame extractor for a service (caller must hold lock)
func (r *ServiceRegistry) startExtractorLocked(state *ServiceState) {
	ctx, cancel := context.WithCancel(context.Background())
	state.Cancel = cancel

	// Get node connection
	nodeConn, exists := r.srv.GetNodeConnection(state.NodeID)
	if !exists {
		log.Printf("[ServiceRegistry] Node %s not connected, cannot start extractor for %s", state.NodeID, state.ID)
		return
	}

	// Open bridge to the service
	bridgeID, dataChan, err := nodeConn.OpenBridge(ctx, state.ID, state.URL)
	if err != nil {
		log.Printf("[ServiceRegistry] Failed to open bridge for %s: %v", state.ID, err)
		return
	}

	log.Printf("[ServiceRegistry] Bridge %s opened for service %s", bridgeID, state.ID)

	// Create BridgeConn
	bridgeConn := server.NewBridgeConn(nodeConn, bridgeID, dataChan)

	// Create media source
	source, err := webrtc.NewMediaSource(state.URL, state.ID, bridgeConn)
	if err != nil {
		log.Printf("[ServiceRegistry] Failed to create media source for %s: %v", state.ID, err)
		bridgeConn.Close()
		ctx := context.Background()
		nodeConn.CloseBridge(ctx, bridgeID)
		return
	}

	// Start producer receive loop (required to pump RTP data from bridge)
	// This must be called after media source creation and before extractor starts
	go func() {
		log.Printf("[ServiceRegistry] Starting producer receive loop for service %s", state.ID)
		if err := source.GetProducer().Start(); err != nil {
			log.Printf("[ServiceRegistry] Producer receive loop ended for service %s: %v", state.ID, err)
		}
	}()

	// Create and start extractor
	extractor := webrtc.NewFrameExtractor(state.ID, r.frameInterval, func(frame *webrtc.Frame) {
		r.storage.Save(state.ID, frame)
	})

	if err := extractor.Start(source); err != nil {
		log.Printf("[ServiceRegistry] Failed to start extractor for %s: %v", state.ID, err)
		source.Close()
		bridgeConn.Close()
		ctx := context.Background()
		nodeConn.CloseBridge(ctx, bridgeID)
		return
	}

	state.Extractor = extractor
	log.Printf("[ServiceRegistry] Started extractor for service %s", state.ID)
}

// stopExtractorLocked stops the frame extractor for a service (caller must hold lock)
func (r *ServiceRegistry) stopExtractorLocked(state *ServiceState) {
	if state.Extractor != nil {
		state.Extractor.Close()
		state.Extractor = nil
	}
	if state.Cancel != nil {
		state.Cancel()
		state.Cancel = nil
	}
	state.Online = false
}

// LoadServices loads all services from the database and syncs with connected nodes
func (r *ServiceRegistry) LoadServices() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// First, sync with currently connected nodes
	if r.srv != nil {
		connectedNodes := r.srv.GetConnectedNodes()
		for _, nodeID := range connectedNodes {
			// Check if node is ready (has sent node_ready)
			if conn, exists := r.srv.GetNodeConnection(nodeID); exists && conn.IsReady() {
				log.Printf("[ServiceRegistry] Node %s is already online/ready", nodeID)
				r.onlineNodes[nodeID] = true
			}
		}
	}

	services, err := r.db.ListAllServices()
	if err != nil {
		return fmt.Errorf("failed to load services: %w", err)
	}

	for _, svc := range services {
		// Call AddService without locking (we already hold the lock)
		r.addServiceLocked(svc)
	}

	log.Printf("[ServiceRegistry] Loaded %d services (%d nodes online)", len(services), len(r.onlineNodes))
	return nil
}

// addServiceLocked adds a service without locking (caller must hold lock)
func (r *ServiceRegistry) addServiceLocked(service *servicev1.Service) error {
	// Check if node is online
	nodeOnline := r.onlineNodes[service.NodeId]

	state := &ServiceState{
		ID:     service.Id,
		Name:   service.Name,
		URL:    service.Url,
		NodeID: service.NodeId,
		Online: nodeOnline,
	}

	r.services[service.Id] = state

	// Initialize node's service set
	if r.nodes[service.NodeId] == nil {
		r.nodes[service.NodeId] = make(map[string]bool)
	}
	r.nodes[service.NodeId][service.Id] = true

	// Start extractor if node is online
	if nodeOnline {
		r.startExtractorLocked(state)
	}

	log.Printf("[ServiceRegistry] Added service %s (node=%s, online=%v)", service.Id, service.NodeId, nodeOnline)
	return nil
}

// Close stops all extractors and cleans up resources
func (r *ServiceRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("[ServiceRegistry] Closing all extractors")

	for _, state := range r.services {
		r.stopExtractorLocked(state)
	}
}
