package relay

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/unblink/unblink/node"
	"github.com/unblink/unblink/relay/cv"
	"github.com/unblink/unblink/relay/realtime"
)

// Relay manages node connections and the service registry
type Relay struct {
	nodes     map[string]*NodeConn // node_id -> connection
	nodesMu   sync.RWMutex
	services  *ServiceRegistry
	db        *Database
	nodeTable *NodeTable
	config    *Config // Centralized configuration
	shutdown  chan struct{}
	wg        sync.WaitGroup

	// Realtime streaming and CV processing
	realtimeStreamManager *realtime.RealtimeStreamManager
	cvEventBus            *cv.CVEventBus
	cvWorkerRegistry      *cv.CVWorkerRegistry
	storageManager        *cv.StorageManager
}

// NewRelay creates a new relay server
func NewRelay() *Relay {
	// Load centralized configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("[Relay] Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := NewDatabase(config.DatabasePath)
	if err != nil {
		log.Fatalf("[Relay] Failed to initialize database: %v", err)
	}

	relay := &Relay{
		nodes:     make(map[string]*NodeConn),
		services:  NewServiceRegistry(),
		shutdown:  make(chan struct{}),
		db:        db,
		nodeTable: NewNodeTable(db.DB),
		config:    config,
	}

	// Initialize CV and realtime streaming subsystems
	relay.initializeCV()

	return relay
}

// initializeCV initializes the CV processing subsystems
func (r *Relay) initializeCV() {
	// Use centralized configuration
	config := r.config

	// Initialize CV event bus first (no dependencies)
	r.cvEventBus = cv.NewCVEventBus()

	// Initialize worker registry (needs event bus, but not storage manager yet)
	r.cvWorkerRegistry = cv.NewCVWorkerRegistry(r.cvEventBus, nil)

	// Initialize storage manager (needs worker registry for auth)
	r.storageManager = cv.NewStorageManager(config.StorageDir, r.cvWorkerRegistry)

	// Set storage manager reference in worker registry (complete the circular dependency)
	r.cvWorkerRegistry.SetStorageManager(r.storageManager)

	// Initialize realtime stream manager
	nodeConnGetter := func(nodeID string) realtime.NodeConn {
		nc := r.GetNode(nodeID)
		if nc == nil {
			return nil
		}
		return nc
	}

	bridgeProxyFactory := func(nc realtime.NodeConn, bridgeID string, service node.Service) (realtime.BridgeTCPProxy, error) {
		nodeConn, ok := nc.(*NodeConn)
		if !ok {
			return nil, fmt.Errorf("invalid node connection type")
		}
		return NewBridgeTCPProxy(nodeConn, bridgeID, service)
	}

	r.realtimeStreamManager = realtime.NewRealtimeStreamManager(nodeConnGetter, bridgeProxyFactory)

	// Register callback for when realtime streams are created
	r.realtimeStreamManager.OnStreamCreated(func(stream *realtime.RealtimeStream) {
		// Get service from stream
		service := stream.GetService().(node.Service)

		// Start frame extraction
		frameExtractor := cv.NewCVFrameExtractor(service.ID, config.FrameInterval, r.cvEventBus, r.storageManager, config.BatchSize)
		if err := frameExtractor.Start(stream); err != nil {
			log.Printf("[Relay] Failed to start frame extractor for service %s: %v", service.ID, err)
			return
		}

		// Attach frame extractor to stream for cleanup
		stream.SetFrameExtractor(frameExtractor)

		log.Printf("[Relay] Started frame extraction for service %s (batchSize=%d)", service.ID, config.BatchSize)
	})

	// Start worker API server
	go func() {
		if err := StartWorkerAPIServer(config.EventPort, r.cvWorkerRegistry, r.storageManager); err != nil {
			log.Printf("[Relay] Worker API server error: %v", err)
		}
	}()

	log.Printf("[Relay] Initialized CV processing (storage=%s, frameInterval=%v, eventPort=%s)",
		config.StorageDir, config.FrameInterval, config.EventPort)
}


// registerNode adds a node to the registry
func (r *Relay) registerNode(nodeID string, nc *NodeConn) {
	r.nodesMu.Lock()
	defer r.nodesMu.Unlock()

	// Remove old connection if exists (but not if it's the same connection)
	if old, exists := r.nodes[nodeID]; exists && old != nc {
		old.Close()
	}

	r.nodes[nodeID] = nc
	nc.nodeID = nodeID
	log.Printf("[Relay] Node registered: %s", nodeID)
}

// removeNode removes a node and its services
func (r *Relay) removeNode(nc *NodeConn) {
	if nc.nodeID == "" {
		return
	}

	r.nodesMu.Lock()
	delete(r.nodes, nc.nodeID)
	r.nodesMu.Unlock()

	// Close the node connection (this also closes all bridges)
	nc.Close()

	// Remove all services from this node
	r.services.RemoveByNode(nc.nodeID)

	// Stop realtime streams for this node
	if r.realtimeStreamManager != nil {
		r.realtimeStreamManager.OnNodeDisconnected(nc.nodeID)
	}

	log.Printf("[Relay] Node removed: %s", nc.nodeID)
}

// GetNode returns a node connection by ID
func (r *Relay) GetNode(nodeID string) *NodeConn {
	r.nodesMu.RLock()
	defer r.nodesMu.RUnlock()
	return r.nodes[nodeID]
}

// ListNodes returns a list of connected node IDs
func (r *Relay) ListNodes() []string {
	r.nodesMu.RLock()
	defer r.nodesMu.RUnlock()

	result := make([]string, 0, len(r.nodes))
	for id := range r.nodes {
		result = append(result, id)
	}
	return result
}

// Services returns the service registry
func (r *Relay) Services() *ServiceRegistry {
	return r.services
}

// Shutdown gracefully shuts down the relay
func (r *Relay) Shutdown() {
	close(r.shutdown)

	// Close all node connections
	r.nodesMu.Lock()
	for _, nc := range r.nodes {
		nc.Close()
	}
	r.nodesMu.Unlock()

	r.wg.Wait()
	log.Printf("[Relay] Shutdown complete")
}

// SendTokenToNode sends an AUTH_TOKEN message to a node
func (r *Relay) SendTokenToNode(nodeID, token string) {
	r.nodesMu.RLock()
	nc, exists := r.nodes[nodeID]
	r.nodesMu.RUnlock()

	if !exists {
		log.Printf("[Relay] Node %s not connected", nodeID)
		return
	}

	// Send AUTH_TOKEN message with the token
	msg := node.NewAuthTokenMsg(uuid.New().String(), token)
	if err := nc.conn.WriteMessage(msg); err != nil {
		log.Printf("[Relay] Failed to send token to node %s: %v", nodeID, err)
		return
	}

	log.Printf("[Relay] Sent token to node %s", nodeID)
}
