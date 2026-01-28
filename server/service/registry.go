package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"unblink/database"
	"unblink/server"
	servicev1 "unblink/server/gen/service/v1"
	"unblink/server/webrtc"
)

// ServiceState tracks the state of a service
type ServiceState struct {
	ID      string
	Name    string
	URL     string
	NodeID  string
	Online  bool
	Handler *ServiceHandler // Handles all service operations

	// Reconnection state
	RetryCount      int
	LastRetryTime   time.Time
	SuccessfulSince time.Time // Last time data flowed successfully
}

// reconnectRequest represents a pending reconnection attempt
type reconnectRequest struct {
	serviceID string
	backoff   time.Duration
}

// ServiceRegistry manages all services and their handlers
type ServiceRegistry struct {
	db            *database.Client
	storage       *webrtc.Storage
	batchManager  *webrtc.BatchManager
	frameInterval time.Duration
	srv           *server.Server

	mu          sync.RWMutex
	services    map[string]*ServiceState   // serviceID -> ServiceState
	nodes       map[string]map[string]bool // nodeID -> set of serviceIDs
	onlineNodes map[string]bool            // nodeID -> online status

	// Idle monitoring
	idleTimeout    time.Duration // How long before considering a bridge idle
	maxRetries     int           // Maximum reconnection attempts
	monitorStop    chan struct{}
	monitorStopped chan struct{}

	// Reconnection queue
	reconnectQueue chan reconnectRequest // Async reconnection requests
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(db *database.Client, frameInterval time.Duration, storage *webrtc.Storage, srv *server.Server, batchMgr *webrtc.BatchManager, idleTimeout time.Duration, maxRetries int) *ServiceRegistry {
	// Wire up callback to save frame metadata to database when frames are saved to disk
	storage.SetOnSaved(func(serviceID, frameID, framePath string, timestamp time.Time, fileSize int64) {
		metadata := &database.FrameMetadata{}
		if err := db.SaveStorageItem(serviceID, framePath, timestamp, fileSize, database.StorageTypeFrame, "image/jpeg", metadata); err != nil {
			log.Printf("[ServiceRegistry] Failed to save frame metadata: %v", err)
		}
	})

	r := &ServiceRegistry{
		db:             db,
		storage:        storage,
		batchManager:   batchMgr,
		frameInterval:  frameInterval,
		srv:            srv,
		services:       make(map[string]*ServiceState),
		nodes:          make(map[string]map[string]bool),
		onlineNodes:    make(map[string]bool),
		idleTimeout:    idleTimeout,
		maxRetries:     maxRetries,
		monitorStop:    make(chan struct{}),
		monitorStopped: make(chan struct{}),
		reconnectQueue: make(chan reconnectRequest, 100), // Buffered channel
	}

	// Start idle monitoring goroutine
	go r.monitorIdleConnections()

	// Start reconnection worker
	go r.reconnectionWorker()

	return r
}

// SetServer sets the server reference (needed after server is created)
func (r *ServiceRegistry) SetServer(srv *server.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.srv = srv
}

// AddService adds a service to the registry and starts handler if node is online
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

	// Start handler if node is online
	if nodeOnline {
		r.startHandlerLocked(state)
	}

	log.Printf("[ServiceRegistry] Added service %s (node=%s, online=%v)", service.Id, service.NodeId, nodeOnline)
	return nil
}

// RemoveService removes a service from the registry and stops its handler
func (r *ServiceRegistry) RemoveService(serviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, exists := r.services[serviceID]
	if !exists {
		return
	}

	// Stop handler (this also calls batchManager.RemoveService)
	r.stopHandlerLocked(state)

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
		r.addServiceLocked(service)
		return
	}

	// Stop old handler
	r.stopHandlerLocked(state)

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

	// Restart handler if node is online
	state.Online = r.onlineNodes[state.NodeID]
	if state.Online {
		r.startHandlerLocked(state)
	}

	log.Printf("[ServiceRegistry] Updated service %s", service.Id)
}

// SetNodeOnline sets a node as online and starts handlers for all its services
func (r *ServiceRegistry) SetNodeOnline(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.onlineNodes[nodeID] = true

	serviceIDs := r.nodes[nodeID]
	if serviceIDs == nil {
		return
	}

	log.Printf("[ServiceRegistry] Node %s online, starting %d handlers", nodeID, len(serviceIDs))

	for serviceID := range serviceIDs {
		state := r.services[serviceID]
		if state != nil && !state.Online {
			state.Online = true
			r.startHandlerLocked(state)
		}
	}
}

// SetNodeOffline sets a node as offline and stops handlers for all its services
func (r *ServiceRegistry) SetNodeOffline(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.onlineNodes, nodeID)

	serviceIDs := r.nodes[nodeID]
	if serviceIDs == nil {
		return
	}

	log.Printf("[ServiceRegistry] Node %s offline, stopping %d handlers", nodeID, len(serviceIDs))

	for serviceID := range serviceIDs {
		state := r.services[serviceID]
		if state != nil && state.Online {
			state.Online = false
			r.stopHandlerLocked(state)
		}
	}
}

// startHandlerLocked starts the service handler for a service (caller must hold lock)
func (r *ServiceRegistry) startHandlerLocked(state *ServiceState) {
	// Create handler with configuration
	handler := NewServiceHandler(ServiceHandlerConfig{
		ServiceID:     state.ID,
		URL:           state.URL,
		NodeID:        state.NodeID,
		FrameInterval: r.frameInterval,
		Storage:       r.storage,
		BatchManager:  r.batchManager,
		Server:        r.srv,
	})

	// Start the handler
	if err := handler.Start(); err != nil {
		log.Printf("[ServiceRegistry] Failed to start handler for %s: %v", state.ID, err)
		return
	}

	state.Handler = handler
	state.SuccessfulSince = time.Now() // Mark as successful
	log.Printf("[ServiceRegistry] Started handler for service %s", state.ID)
}

// stopHandlerLocked stops the service handler for a service (caller must hold lock)
func (r *ServiceRegistry) stopHandlerLocked(state *ServiceState) {
	if state.Handler != nil {
		state.Handler.Stop()
		state.Handler = nil
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

	// Start handler if node is online
	if nodeOnline {
		r.startHandlerLocked(state)
	}

	log.Printf("[ServiceRegistry] Added service %s (node=%s, online=%v)", service.Id, service.NodeId, nodeOnline)
	return nil
}

// Close stops all handlers and cleans up resources
func (r *ServiceRegistry) Close() {
	// Stop idle monitoring first
	close(r.monitorStop)
	<-r.monitorStopped // Wait for monitor to finish

	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("[ServiceRegistry] Closing all handlers")

	for _, state := range r.services {
		r.stopHandlerLocked(state)
	}
}

// monitorIdleConnections periodically checks for idle bridges and attempts reconnection
func (r *ServiceRegistry) monitorIdleConnections() {
	defer close(r.monitorStopped)

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	log.Printf("[ServiceRegistry] Started idle connection monitor (timeout=%v, maxRetries=%d)", r.idleTimeout, r.maxRetries)

	for {
		select {
		case <-r.monitorStop:
			log.Printf("[ServiceRegistry] Stopping idle connection monitor")
			return

		case <-ticker.C:
			r.checkIdleConnections()
		}
	}
}

// checkIdleConnections checks all services for idle bridges
func (r *ServiceRegistry) checkIdleConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, state := range r.services {
		if !state.Online || state.Handler == nil {
			continue
		}

		bridgeConn := state.Handler.GetBridgeConn()
		if bridgeConn == nil {
			continue
		}

		idleDuration := bridgeConn.IdleDuration()

		// Check if bridge is idle
		if idleDuration > r.idleTimeout {
			log.Printf("[ServiceRegistry] Service %s idle for %v (threshold=%v)", state.ID, idleDuration, r.idleTimeout)

			// Check retry limit
			if state.RetryCount >= r.maxRetries {
				log.Printf("[ServiceRegistry] Service %s exceeded max retries (%d/%d), giving up", state.ID, state.RetryCount, r.maxRetries)
				r.stopHandlerLocked(state)
				continue
			}

			// Attempt reconnection
			log.Printf("[ServiceRegistry] Attempting reconnection for service %s (attempt %d/%d)", state.ID, state.RetryCount+1, r.maxRetries)
			r.reconnectServiceLocked(state)
		} else {
			// Connection is active - check if we should reset retry counter
			if state.RetryCount > 0 && !state.SuccessfulSince.IsZero() {
				// If connection has been successful for longer than idle timeout, reset counter
				successDuration := time.Since(state.SuccessfulSince)
				if successDuration > r.idleTimeout {
					log.Printf("[ServiceRegistry] Service %s stable for %v, resetting retry counter", state.ID, successDuration)
					state.RetryCount = 0
					state.SuccessfulSince = time.Time{} // Clear
				}
			}
		}
	}
}

// reconnectServiceLocked attempts to reconnect a service (caller must hold lock)
func (r *ServiceRegistry) reconnectServiceLocked(state *ServiceState) {
	// Stop current handler
	r.stopHandlerLocked(state)

	// Increment retry count
	state.RetryCount++
	state.LastRetryTime = time.Now()

	// Calculate backoff
	backoff := time.Duration(state.RetryCount) * 2 * time.Second

	// Queue for async reconnection (DON'T RELEASE LOCK!)
	select {
	case r.reconnectQueue <- reconnectRequest{
		serviceID: state.ID,
		backoff:   backoff,
	}:
		log.Printf("[ServiceRegistry] Queued reconnection for %s (backoff=%v)", state.ID, backoff)
	default:
		log.Printf("[ServiceRegistry] Reconnect queue full, dropping request for %s", state.ID)
	}
}

// reconnectionWorker processes queued reconnection requests asynchronously
func (r *ServiceRegistry) reconnectionWorker() {
	for {
		select {
		case <-r.monitorStop:
			log.Printf("[ServiceRegistry] Stopping reconnection worker")
			return

		case req := <-r.reconnectQueue:
			// Sleep with backoff (outside lock!)
			log.Printf("[ServiceRegistry] Waiting %v before reconnecting service %s", req.backoff, req.serviceID)
			time.Sleep(req.backoff)

			// Now acquire lock and attempt reconnection
			r.mu.Lock()
			state, exists := r.services[req.serviceID]
			if !exists {
				r.mu.Unlock()
				log.Printf("[ServiceRegistry] Service %s no longer exists, skipping reconnection", req.serviceID)
				continue
			}

			if !r.onlineNodes[state.NodeID] {
				r.mu.Unlock()
				log.Printf("[ServiceRegistry] Node %s offline, skipping reconnection for service %s", state.NodeID, req.serviceID)
				continue
			}

			// Attempt reconnection
			log.Printf("[ServiceRegistry] Reconnecting service %s to node %s", req.serviceID, state.NodeID)
			r.startHandlerLocked(state)

			if state.Handler != nil && state.Handler.IsRunning() {
				state.SuccessfulSince = time.Now()
				log.Printf("[ServiceRegistry] Successfully reconnected service %s", req.serviceID)
			}
			r.mu.Unlock()
		}
	}
}
