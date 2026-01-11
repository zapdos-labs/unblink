package realtime

import (
	"fmt"
	"log"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/unblink/unblink/node"
	"github.com/unblink/unblink/relay/sources"
)

// RealtimeStream represents a persistent stream for one camera
type RealtimeStream struct {
	id          string
	service     node.Service
	nodeConn    NodeConn
	bridgeID    string
	bridgeProxy BridgeTCPProxy
	source      sources.Source

	// Frame extractor (set externally after creation)
	frameExtractor FrameExtractor

	closeOnce sync.Once
	closeChan chan struct{}
}

// FrameExtractor interface for closing the extractor
type FrameExtractor interface {
	Close()
}

// NodeConn interface for dependency injection
type NodeConn interface {
	OpenBridge(service node.Service) (string, error)
	CloseBridge(bridgeID string) error
}

// BridgeTCPProxy interface for dependency injection
type BridgeTCPProxy interface {
	Addr() string
	Close()
}

// NewRealtimeStream creates a new realtime stream
func NewRealtimeStream(id string, service node.Service, nodeConn NodeConn, newBridgeProxy func(NodeConn, string, node.Service) (BridgeTCPProxy, error)) (*RealtimeStream, error) {
	// Open bridge through node
	bridgeID, err := nodeConn.OpenBridge(service)
	if err != nil {
		return nil, fmt.Errorf("open bridge: %w", err)
	}

	log.Printf("[RealtimeStream] Opened bridge %s for service %s", bridgeID, service.ID)

	// Create TCP proxy for the bridge
	bridgeProxy, err := newBridgeProxy(nodeConn, bridgeID, service)
	if err != nil {
		nodeConn.CloseBridge(bridgeID)
		return nil, fmt.Errorf("create bridge proxy: %w", err)
	}

	proxyAddr := bridgeProxy.Addr()
	log.Printf("[RealtimeStream] Bridge proxy listening on: %s", proxyAddr)

	// Create source (RTSP or MJPEG)
	source, err := sources.New(service, proxyAddr)
	if err != nil {
		bridgeProxy.Close()
		nodeConn.CloseBridge(bridgeID)
		return nil, fmt.Errorf("create source: %w", err)
	}

	stream := &RealtimeStream{
		id:          id,
		service:     service,
		nodeConn:    nodeConn,
		bridgeID:    bridgeID,
		bridgeProxy: bridgeProxy,
		source:      source,
		closeChan:   make(chan struct{}),
	}

	log.Printf("[RealtimeStream] Created stream %s for service %s (type=%s)", id, service.ID, service.Type)

	return stream, nil
}

// GetReceivers returns the H.264 receivers for CV processing
func (s *RealtimeStream) GetReceivers() []*core.Receiver {
	return s.source.GetReceivers()
}

// GetProducer returns the producer for this stream
func (s *RealtimeStream) GetProducer() core.Producer {
	return s.source.GetProducer()
}

// GetService returns the service this stream is connected to
func (s *RealtimeStream) GetService() interface{} {
	return s.service
}

// GetID returns the stream ID
func (s *RealtimeStream) GetID() string {
	return s.id
}

// SetFrameExtractor sets the frame extractor for this stream
func (s *RealtimeStream) SetFrameExtractor(extractor FrameExtractor) {
	s.frameExtractor = extractor
}

// Close closes the stream and cleans up resources
func (s *RealtimeStream) Close() {
	s.closeOnce.Do(func() {
		log.Printf("[RealtimeStream] Closing stream %s", s.id)

		close(s.closeChan)

		// Close frame extractor first to stop FFmpeg
		if s.frameExtractor != nil {
			s.frameExtractor.Close()
		}

		if s.bridgeProxy != nil {
			s.bridgeProxy.Close()
		}

		if s.source != nil {
			s.source.Close()
		}

		if s.nodeConn != nil {
			s.nodeConn.CloseBridge(s.bridgeID)
		}

		log.Printf("[RealtimeStream] Closed stream %s", s.id)
	})
}

// CloseChan returns a channel that is closed when the stream is closed
func (s *RealtimeStream) CloseChan() <-chan struct{} {
	return s.closeChan
}
