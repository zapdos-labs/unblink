package server

import (
	"fmt"
	"sync"
	"time"
)

// NodeInfo holds information about a connected node
type NodeInfo struct {
	NodeID       string
	Hostname     string
	MACAddresses []string
	ConnectedAt  time.Time
	LastSeen     time.Time
}

// NodeStore provides in-memory node storage
type NodeStore struct {
	nodes map[string]*NodeInfo
	mu    sync.RWMutex
}

// NewNodeStore creates a new node store
func NewNodeStore() *NodeStore {
	return &NodeStore{
		nodes: make(map[string]*NodeInfo),
	}
}

// ValidateToken validates a JWT token and returns the node ID
func (ns *NodeStore) ValidateToken(jwtSecret, tokenString string) (string, error) {
	claims, err := ValidateNodeToken(tokenString, jwtSecret)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	return claims.NodeID, nil
}

// RegisterNode stores or updates node information
func (ns *NodeStore) RegisterNode(nodeID, hostname string, macs []string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	now := time.Now()

	// Check if node already exists
	if existing, ok := ns.nodes[nodeID]; ok {
		// Update existing node
		existing.Hostname = hostname
		existing.MACAddresses = macs
		existing.LastSeen = now
		return nil
	}

	// Create new node entry
	ns.nodes[nodeID] = &NodeInfo{
		NodeID:       nodeID,
		Hostname:     hostname,
		MACAddresses: macs,
		ConnectedAt:  now,
		LastSeen:     now,
	}

	return nil
}

// GetNode retrieves node information by ID
func (ns *NodeStore) GetNode(nodeID string) (*NodeInfo, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	node, ok := ns.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	// Return a copy to avoid external modifications
	copy := *node
	return &copy, nil
}

// UpdateLastSeen updates the last seen timestamp for a node
func (ns *NodeStore) UpdateLastSeen(nodeID string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	node, ok := ns.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.LastSeen = time.Now()
	return nil
}

// RemoveNode removes a node from the store
func (ns *NodeStore) RemoveNode(nodeID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	delete(ns.nodes, nodeID)
}

// GetAllNodes returns all registered nodes
func (ns *NodeStore) GetAllNodes() []*NodeInfo {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	result := make([]*NodeInfo, 0, len(ns.nodes))
	for _, node := range ns.nodes {
		copy := *node
		result = append(result, &copy)
	}

	return result
}
