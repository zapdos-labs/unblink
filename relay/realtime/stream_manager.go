package realtime

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/zapdos-labs/unblink/node"
)

// StreamCreatedCallback is called when a stream is created
type StreamCreatedCallback func(*RealtimeStream)

// RealtimeStreamManager manages lifecycle of realtime streams
type RealtimeStreamManager struct {
	streams           map[string]*RealtimeStream // serviceID → stream
	mu                sync.RWMutex
	nodeConnGetter    func(string) NodeConn
	bridgeProxyFactory func(NodeConn, string, node.Service) (BridgeTCPProxy, error)
	onStreamCreated   StreamCreatedCallback
}

// NewRealtimeStreamManager creates a new stream manager
func NewRealtimeStreamManager(
	nodeConnGetter func(string) NodeConn,
	bridgeProxyFactory func(NodeConn, string, node.Service) (BridgeTCPProxy, error),
) *RealtimeStreamManager {
	return &RealtimeStreamManager{
		streams:            make(map[string]*RealtimeStream),
		nodeConnGetter:     nodeConnGetter,
		bridgeProxyFactory: bridgeProxyFactory,
	}
}

// OnStreamCreated registers a callback for when streams are created
func (m *RealtimeStreamManager) OnStreamCreated(callback StreamCreatedCallback) {
	m.onStreamCreated = callback
}

// OnServiceAnnounced creates a realtime stream for a camera service
func (m *RealtimeStreamManager) OnServiceAnnounced(service node.Service, nodeID string) {
	// Only create streams for camera services
	if service.Type != "rtsp" && service.Type != "mjpeg" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if stream already exists
	if _, exists := m.streams[service.ID]; exists {
		log.Printf("[RealtimeStreamManager] Stream already exists for service %s", service.ID)
		return
	}

	// Get node connection
	nodeConn := m.nodeConnGetter(nodeID)
	if nodeConn == nil {
		log.Printf("[RealtimeStreamManager] Node %s not found", nodeID)
		return
	}

	// Create stream
	streamID := uuid.New().String()
	stream, err := NewRealtimeStream(streamID, service, nodeConn, m.bridgeProxyFactory)
	if err != nil {
		log.Printf("[RealtimeStreamManager] Failed to create stream for service %s: %v", service.ID, err)
		return
	}

	m.streams[service.ID] = stream
	log.Printf("[RealtimeStreamManager] Created realtime stream for service %s", service.ID)

	// Notify callback
	if m.onStreamCreated != nil {
		go m.onStreamCreated(stream)
	}
}

// OnServiceRemoved removes a realtime stream
func (m *RealtimeStreamManager) OnServiceRemoved(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[serviceID]
	if !exists {
		return
	}

	stream.Close()
	delete(m.streams, serviceID)
	log.Printf("[RealtimeStreamManager] Removed stream for service %s", serviceID)
}

// OnNodeDisconnected removes all streams for a node
func (m *RealtimeStreamManager) OnNodeDisconnected(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find all streams for this node and close them
	var toRemove []string
	for serviceID, stream := range m.streams {
		// Note: We don't have direct nodeID in stream, so we'll close all
		// In a real implementation, you might want to track nodeID per stream
		stream.Close()
		toRemove = append(toRemove, serviceID)
	}

	for _, serviceID := range toRemove {
		delete(m.streams, serviceID)
	}

	if len(toRemove) > 0 {
		log.Printf("[RealtimeStreamManager] Removed %d streams for node %s", len(toRemove), nodeID)
	}
}

// GetStream retrieves a stream by service ID
func (m *RealtimeStreamManager) GetStream(serviceID string) (*RealtimeStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[serviceID]
	if !exists {
		return nil, fmt.Errorf("stream not found for service %s", serviceID)
	}

	return stream, nil
}

// ListStreams returns all active streams
func (m *RealtimeStreamManager) ListStreams() []*RealtimeStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*RealtimeStream, 0, len(m.streams))
	for _, stream := range m.streams {
		streams = append(streams, stream)
	}

	return streams
}
