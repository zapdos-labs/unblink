package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"unb/server"
	"unb/server/webrtc"
)

// ServiceHandler encapsulates all components needed to handle an active service
// It manages the bridge connection, media source, and frame extraction for one service
type ServiceHandler struct {
	serviceID string
	url       string
	nodeID    string

	// Service components
	bridgeConn  *server.BridgeConn
	mediaSource webrtc.MediaSource
	extractor   *webrtc.FrameExtractor

	// Shared infrastructure (injected)
	storage      *webrtc.Storage
	batchManager *webrtc.BatchManager
	srv          *server.Server

	// Configuration
	frameInterval time.Duration

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// ServiceHandlerConfig holds configuration for creating a service handler
type ServiceHandlerConfig struct {
	ServiceID     string
	URL           string
	NodeID        string
	FrameInterval time.Duration
	Storage       *webrtc.Storage
	BatchManager  *webrtc.BatchManager
	Server        *server.Server
}

// NewServiceHandler creates a new service handler
func NewServiceHandler(cfg ServiceHandlerConfig) *ServiceHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceHandler{
		serviceID:     cfg.ServiceID,
		url:           cfg.URL,
		nodeID:        cfg.NodeID,
		frameInterval: cfg.FrameInterval,
		storage:       cfg.Storage,
		batchManager:  cfg.BatchManager,
		srv:           cfg.Server,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start initializes and starts all service components
func (h *ServiceHandler) Start() error {
	// Get node connection
	nodeConn, exists := h.srv.GetNodeConnection(h.nodeID)
	if !exists {
		return fmt.Errorf("node %s not connected", h.nodeID)
	}

	// Open bridge to the service
	bridgeID, dataChan, err := nodeConn.OpenBridge(h.ctx, h.serviceID, h.url)
	if err != nil {
		return fmt.Errorf("failed to open bridge: %w", err)
	}

	log.Printf("[ServiceHandler] Bridge %s opened for service %s", bridgeID, h.serviceID)

	// Create BridgeConn
	h.bridgeConn = server.NewBridgeConn(nodeConn, bridgeID, dataChan)

	// Create media source
	h.mediaSource, err = webrtc.NewMediaSource(h.url, h.serviceID, h.bridgeConn)
	if err != nil {
		h.bridgeConn.Close()
		ctx := context.Background()
		nodeConn.CloseBridge(ctx, bridgeID)
		return fmt.Errorf("failed to create media source: %w", err)
	}

	// Start producer receive loop (required to pump RTP data from bridge)
	go func() {
		log.Printf("[ServiceHandler] Starting producer receive loop for service %s", h.serviceID)
		if err := h.mediaSource.GetProducer().Start(); err != nil {
			log.Printf("[ServiceHandler] Producer receive loop ended for service %s: %v", h.serviceID, err)
		}
	}()

	// Create and start extractor
	h.extractor = webrtc.NewFrameExtractor(h.serviceID, h.frameInterval, func(frame *webrtc.Frame) {
		// Save frame to disk
		h.storage.Save(h.serviceID, frame)

		// Add frame to batch manager for vLLM processing
		if h.batchManager != nil {
			h.batchManager.AddFrame(frame)
		}
	})

	if err := h.extractor.Start(h.mediaSource); err != nil {
		h.mediaSource.Close()
		h.bridgeConn.Close()
		ctx := context.Background()
		nodeConn.CloseBridge(ctx, bridgeID)
		return fmt.Errorf("failed to start extractor: %w", err)
	}

	log.Printf("[ServiceHandler] Started handler for service %s", h.serviceID)
	return nil
}

// Stop stops all service components and cleans up resources
func (h *ServiceHandler) Stop() {
	log.Printf("[ServiceHandler] Stopping handler for service %s", h.serviceID)

	// Stop extractor
	if h.extractor != nil {
		h.extractor.Close()
		h.extractor = nil
	}

	// Close media source
	if h.mediaSource != nil {
		h.mediaSource.Close()
		h.mediaSource = nil
	}

	// Close bridge connection
	if h.bridgeConn != nil {
		h.bridgeConn.Close()
		h.bridgeConn = nil
	}

	// Clean up batch manager context
	if h.batchManager != nil {
		h.batchManager.RemoveService(h.serviceID)
	}

	// Cancel context
	if h.cancel != nil {
		h.cancel()
	}
}

// GetBridgeConn returns the bridge connection (for idle monitoring)
func (h *ServiceHandler) GetBridgeConn() *server.BridgeConn {
	return h.bridgeConn
}

// IsRunning returns true if the handler is currently running
func (h *ServiceHandler) IsRunning() bool {
	return h.bridgeConn != nil && h.extractor != nil
}
