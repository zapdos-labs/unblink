package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// NodeEventCallback is a callback function for node events
type NodeEventCallback func(nodeID string)

// Server manages all node connections
type Server struct {
	config    *Config
	nodeStore *NodeStore
	nodes     map[string]*NodeConn
	nodesMu   sync.Mutex

	// Node event callbacks
	nodeReadyCallbacks   []NodeEventCallback
	nodeOfflineCallbacks []NodeEventCallback
}

// NewServer creates a new Server instance
func NewServer(cfg *Config) *Server {
	return &Server{
		config:    cfg,
		nodeStore: NewNodeStore(),
		nodes:     make(map[string]*NodeConn),
	}
}

// GetConfig returns the server configuration
func (s *Server) GetConfig() *Config {
	return s.config
}

// RegisterNodeConnection registers a node connection
func (s *Server) RegisterNodeConnection(nodeID string, conn *NodeConn) {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	s.nodes[nodeID] = conn
	log.Printf("[Server] Node %s connected", nodeID)
}

// UnregisterNodeConnection unregisters a node connection
func (s *Server) UnregisterNodeConnection(nodeID string) {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	delete(s.nodes, nodeID)
	log.Printf("[Server] Node %s disconnected", nodeID)
}

// GetNodeConnection gets a node connection by ID
func (s *Server) GetNodeConnection(nodeID string) (*NodeConn, bool) {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	conn, ok := s.nodes[nodeID]
	return conn, ok
}

// GetConnectedNodes returns a list of all connected node IDs
func (s *Server) GetConnectedNodes() []string {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	var nodeIDs []string
	for nodeID := range s.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}

// OnNodeReady registers a callback for when a node becomes ready
func (s *Server) OnNodeReady(callback NodeEventCallback) {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	s.nodeReadyCallbacks = append(s.nodeReadyCallbacks, callback)
}

// OnNodeOffline registers a callback for when a node goes offline
func (s *Server) OnNodeOffline(callback NodeEventCallback) {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()
	s.nodeOfflineCallbacks = append(s.nodeOfflineCallbacks, callback)
}

// notifyNodeReady notifies all registered callbacks that a node is ready
func (s *Server) notifyNodeReady(nodeID string) {
	s.nodesMu.Lock()
	callbacks := make([]NodeEventCallback, len(s.nodeReadyCallbacks))
	copy(callbacks, s.nodeReadyCallbacks)
	s.nodesMu.Unlock()

	for _, cb := range callbacks {
		go cb(nodeID)
	}
}

// notifyNodeOffline notifies all registered callbacks that a node is offline
func (s *Server) notifyNodeOffline(nodeID string) {
	s.nodesMu.Lock()
	callbacks := make([]NodeEventCallback, len(s.nodeOfflineCallbacks))
	copy(callbacks, s.nodeOfflineCallbacks)
	s.nodesMu.Unlock()

	for _, cb := range callbacks {
		go cb(nodeID)
	}
}

// UpgradeToWebSocket upgrades an HTTP connection to WebSocket
func (s *Server) UpgradeToWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in simplified version
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}

	return conn, nil
}
