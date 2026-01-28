package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"unblink/shared"
)

// Bridge represents an active bridge connection
type Bridge struct {
	BridgeID  string
	ServiceID string
	CreatedAt time.Time
}

// NodeConn handles a single node connection using CBOR messages
type NodeConn struct {
	wsConn       *websocket.Conn
	transport    MessageTransport
	server       *Server
	nodeID       string
	hostname     string
	macAddresses []string
	shutdown     chan struct{}
	closed       bool
	closeMu      sync.Mutex
	ready        int32 // atomic: 0 = not ready, 1 = ready

	// Bridge management
	bridges     map[string]*Bridge     // bridgeID -> Bridge
	bridgeChans map[string]chan []byte // bridgeID -> data channel
	bridgeMu    sync.RWMutex

	// Request/response correlation
	pendingRequests   map[uint64]chan any
	pendingRequestsMu sync.Mutex
	nextMessageID     uint64
	messageIDMu       sync.Mutex
}

// NewNodeConn creates a new node connection from a WebSocket connection
func NewNodeConn(wsConn *websocket.Conn, server *Server) *NodeConn {
	return &NodeConn{
		wsConn:          wsConn,
		server:          server,
		transport:       NewMessageTransport(wsConn),
		shutdown:        make(chan struct{}),
		bridges:         make(map[string]*Bridge),
		bridgeChans:     make(map[string]chan []byte),
		pendingRequests: make(map[uint64]chan any),
	}
}

// Run starts the message loop for handling the node connection
func (nc *NodeConn) Run() error {
	log.Printf("[NodeConn] Starting message loop...")

	go nc.messageLoop()
	go nc.monitorConnection()

	return nil
}

func (nc *NodeConn) monitorConnection() {
	select {
	case <-nc.shutdown:
		log.Printf("[NodeConn %s] Shutdown requested", nc.nodeID)
		nc.Close()
	}
}

func (nc *NodeConn) messageLoop() {
	for {
		select {
		case <-nc.shutdown:
			return
		default:
		}

		msg, err := nc.transport.ReadMessage()
		if err != nil {
			log.Printf("[NodeConn] Read error: %v", err)
			nc.Close()
			return
		}

		if err := nc.handleMessage(msg); err != nil {
			log.Printf("[NodeConn] Error handling message: %v", err)
		}
	}
}

func (nc *NodeConn) handleMessage(msg any) error {
	m, err := shared.GetMessager(msg)
	if err != nil {
		return fmt.Errorf("invalid message type: %w", err)
	}

	msgType := m.GetType()

	switch msgType {
	case shared.MessageTypeTokenCheckRequest:
		return nc.handleTokenCheckRequest(msg.(*shared.TokenCheckRequest))

	case shared.MessageTypeNewTokenRequest:
		return nc.handleNewTokenRequest(msg.(*shared.NewTokenRequest))

	case shared.MessageTypeRegisterRequest:
		return nc.handleRegisterRequest(msg.(*shared.RegisterRequest))

	case shared.MessageTypeNodeReady:
		return nc.handleNodeReady(msg.(*shared.NodeReadyMessage))

	case shared.MessageTypeOpenBridgeResponse:
		return nc.handleOpenBridgeResponse(msg.(*shared.OpenBridgeResponse))

	case shared.MessageTypeCloseBridgeResponse:
		return nc.handleCloseBridgeResponse(msg.(*shared.CloseBridgeResponse))

	case shared.MessageTypeBridgeData:
		return nc.handleBridgeData(msg.(*shared.BridgeDataMessage))

	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

// ============================================================
// Token Handlers
// ============================================================

func (nc *NodeConn) handleTokenCheckRequest(req *shared.TokenCheckRequest) error {
	log.Printf("[NodeConn] Token check request: nodeID=%s", req.NodeID)

	// Validate the token
	claims, err := ValidateNodeToken(req.Token, nc.server.config.JWTSecret)
	valid := err == nil && claims != nil && claims.NodeID == req.NodeID

	if valid {
		log.Printf("[NodeConn] Token valid for node %s", req.NodeID)
	} else {
		log.Printf("[NodeConn] Token invalid for node %s: %v", req.NodeID, err)
	}

	resp := &shared.TokenCheckResponse{
		Message: shared.Message{
			Type: shared.MessageTypeTokenCheckResponse,
			ID:   req.ID,
		},
		Valid: valid,
	}

	return nc.transport.WriteMessage(resp)
}

func (nc *NodeConn) handleNewTokenRequest(req *shared.NewTokenRequest) error {
	log.Printf("[NodeConn] New token request: nodeID=%s, hostname=%q", req.NodeID, req.Hostname)

	token, err := GenerateNodeToken(req.NodeID, nc.server.config.JWTSecret)
	if err != nil {
		log.Printf("[NodeConn] Failed to generate token: %v", err)
		resp := &shared.NewTokenResponse{
			Message: shared.Message{
				Type:  shared.MessageTypeNewTokenResponse,
				ID:    req.ID,
				Error: err.Error(),
			},
		}
		return nc.transport.WriteMessage(resp)
	}

	log.Printf("[NodeConn] Generated new token for node %s", req.NodeID)

	resp := &shared.NewTokenResponse{
		Message: shared.Message{
			Type: shared.MessageTypeNewTokenResponse,
			ID:   req.ID,
		},
		Token: token,
	}

	return nc.transport.WriteMessage(resp)
}

// ============================================================
// Registration Handlers
// ============================================================

func (nc *NodeConn) handleRegisterRequest(req *shared.RegisterRequest) error {
	log.Printf("[NodeConn] Register request: nodeID=%s, hostname=%q", req.NodeID, req.Hostname)

	// Validate the token
	claims, err := ValidateNodeToken(req.Token, nc.server.config.JWTSecret)
	if err != nil || claims == nil || claims.NodeID != req.NodeID {
		log.Printf("[NodeConn] Invalid token for registration: %v", err)
		resp := &shared.RegisterResponse{
			Message: shared.Message{
				Type:  shared.MessageTypeRegisterResponse,
				ID:    req.ID,
				Error: "invalid token",
			},
			Success: false,
		}
		return nc.transport.WriteMessage(resp)
	}

	// Store node info
	nc.nodeID = req.NodeID
	nc.hostname = req.Hostname
	nc.macAddresses = req.MACAddresses

	// Register in server's node map
	nc.server.RegisterNodeConnection(req.NodeID, nc)

	// Store in node store
	if err := nc.server.nodeStore.RegisterNode(req.NodeID, req.Hostname, req.MACAddresses); err != nil {
		log.Printf("[NodeConn] Failed to register node: %v", err)
	}

	log.Printf("[NodeConn] Node %s registered successfully", req.NodeID)

	dashboardURL := fmt.Sprintf("%s/node/%s", nc.server.config.DashboardURL, req.NodeID)
	log.Printf("[NodeConn] Dashboard URL: %s", dashboardURL)

	resp := &shared.RegisterResponse{
		Message: shared.Message{
			Type: shared.MessageTypeRegisterResponse,
			ID:   req.ID,
		},
		Success:      true,
		DashboardURL: dashboardURL,
	}

	if err := nc.transport.WriteMessage(resp); err != nil {
		return err
	}

	return nil
}

func (nc *NodeConn) handleNodeReady(msg *shared.NodeReadyMessage) error {
	if nc.nodeID == "" {
		log.Printf("[NodeConn] Received node_ready but no nodeID set")
		return nil
	}

	log.Printf("[NodeConn] Node %s is ready for bridges", nc.nodeID)
	atomic.StoreInt32(&nc.ready, 1)

	// Notify server that node is ready
	if nc.server != nil {
		nc.server.notifyNodeReady(nc.nodeID)
	}

	return nil
}

// ============================================================
// Bridge Handlers
// ============================================================

func (nc *NodeConn) handleOpenBridgeResponse(resp *shared.OpenBridgeResponse) error {
	log.Printf("[NodeConn] OpenBridge response: success=%v, msgID=%d", resp.Success, resp.ID)

	nc.pendingRequestsMu.Lock()
	respChan, exists := nc.pendingRequests[resp.ID]
	if exists {
		delete(nc.pendingRequests, resp.ID)
	}
	nc.pendingRequestsMu.Unlock()

	if exists {
		select {
		case respChan <- resp:
		default:
			log.Printf("[NodeConn] Failed to send OpenBridgeResponse to waiting channel")
		}
	}

	return nil
}

func (nc *NodeConn) handleCloseBridgeResponse(resp *shared.CloseBridgeResponse) error {
	log.Printf("[NodeConn] CloseBridge response: success=%v, msgID=%d", resp.Success, resp.ID)

	nc.pendingRequestsMu.Lock()
	respChan, exists := nc.pendingRequests[resp.ID]
	if exists {
		delete(nc.pendingRequests, resp.ID)
	}
	nc.pendingRequestsMu.Unlock()

	if exists {
		select {
		case respChan <- resp:
		default:
			log.Printf("[NodeConn] Failed to send CloseBridgeResponse to waiting channel")
		}
	}

	return nil
}

func (nc *NodeConn) handleBridgeData(msg *shared.BridgeDataMessage) error {
	nc.bridgeMu.RLock()
	dataChan, exists := nc.bridgeChans[msg.BridgeID]
	nc.bridgeMu.RUnlock()

	if !exists {
		log.Printf("[NodeConn] BridgeData for unknown bridge %s, ignoring", msg.BridgeID)
		return nil
	}

	// Non-blocking send to prevent blocking the message loop
	select {
	case dataChan <- msg.Data:
		// Data sent successfully
		// if len(dataChan) > 1500 {
		// 	log.Printf("[NodeConn] Bridge %s channel high utilization: %d/2000", msg.BridgeID, len(dataChan))
		// }
	default:
		// Log suppressed to avoid flooding
		// log.Printf("[NodeConn] Bridge %s data channel full, dropping data", msg.BridgeID)
	}

	return nil
}

// ============================================================
// Bridge Operations
// ============================================================

func (nc *NodeConn) getNextMessageID() uint64 {
	nc.messageIDMu.Lock()
	defer nc.messageIDMu.Unlock()
	nc.nextMessageID++
	return nc.nextMessageID
}

// OpenBridge opens a bridge to a service on the node
func (nc *NodeConn) OpenBridge(ctx context.Context, serviceID, serviceURL string) (string, chan []byte, error) {
	if !nc.isReady() {
		return "", nil, fmt.Errorf("node %s not ready", nc.nodeID)
	}

	// Generate unique bridge ID
	bridgeID := uuid.New().String()

	// Create buffered data channel
	// Increased buffer size to handle video bursts and prevent drops
	// 2000 chunks allows for significant buffering even with small packets
	dataChan := make(chan []byte, 2000)

	// Register the data channel
	nc.bridgeMu.Lock()
	nc.bridgeChans[bridgeID] = dataChan
	nc.bridgeMu.Unlock()

	// Create response channel for correlation
	msgID := nc.getNextMessageID()
	respChan := make(chan any, 1)

	nc.pendingRequestsMu.Lock()
	nc.pendingRequests[msgID] = respChan
	nc.pendingRequestsMu.Unlock()

	// Send OpenBridgeRequest to node
	req := &shared.OpenBridgeRequest{
		Message: shared.Message{
			Type: shared.MessageTypeOpenBridgeRequest,
			ID:   msgID,
		},
		BridgeID:  bridgeID,
		ServiceID: serviceID,
		Service: shared.Service{
			ServiceURL: serviceURL,
		},
	}

	if err := nc.transport.WriteMessage(req); err != nil {
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()

		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)

		return "", nil, fmt.Errorf("send OpenBridgeRequest: %w", err)
	}

	// Wait for response with timeout
	timeout := 30 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-respChan:
		openResp, ok := resp.(*shared.OpenBridgeResponse)
		if !ok {
			return "", nil, fmt.Errorf("invalid response type: %T", resp)
		}

		if !openResp.Success {
			nc.bridgeMu.Lock()
			delete(nc.bridgeChans, bridgeID)
			nc.bridgeMu.Unlock()
			close(dataChan)

			errMsg := openResp.Error
			if errMsg == "" {
				errMsg = "bridge open failed"
			}
			return "", nil, fmt.Errorf("%s", errMsg)
		}

		// Store bridge info
		nc.bridgeMu.Lock()
		nc.bridges[bridgeID] = &Bridge{
			BridgeID:  bridgeID,
			ServiceID: serviceID,
			CreatedAt: time.Now(),
		}
		nc.bridgeMu.Unlock()

		log.Printf("[NodeConn %s] Bridge %s opened successfully for service %s", nc.nodeID, bridgeID, serviceID)
		return bridgeID, dataChan, nil

	case <-timer.C:
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()

		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)

		return "", nil, fmt.Errorf("timeout waiting for OpenBridge response")

	case <-ctx.Done():
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()

		nc.bridgeMu.Lock()
		delete(nc.bridgeChans, bridgeID)
		nc.bridgeMu.Unlock()
		close(dataChan)

		return "", nil, ctx.Err()
	}
}

// CloseBridge closes a bridge
func (nc *NodeConn) CloseBridge(ctx context.Context, bridgeID string) error {
	// Remove from our tracking
	nc.bridgeMu.Lock()
	delete(nc.bridges, bridgeID)
	dataChan, exists := nc.bridgeChans[bridgeID]
	if exists {
		delete(nc.bridgeChans, bridgeID)
	}
	nc.bridgeMu.Unlock()

	if exists {
		close(dataChan)
	}

	// Send CloseBridgeRequest to node
	msgID := nc.getNextMessageID()
	respChan := make(chan any, 1)

	nc.pendingRequestsMu.Lock()
	nc.pendingRequests[msgID] = respChan
	nc.pendingRequestsMu.Unlock()

	req := &shared.CloseBridgeRequest{
		Message: shared.Message{
			Type: shared.MessageTypeCloseBridgeRequest,
			ID:   msgID,
		},
		BridgeID: bridgeID,
	}

	if err := nc.transport.WriteMessage(req); err != nil {
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()
		return fmt.Errorf("send CloseBridgeRequest: %w", err)
	}

	// Wait for response with timeout
	timeout := 10 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-respChan:
		closeResp, ok := resp.(*shared.CloseBridgeResponse)
		if !ok {
			return fmt.Errorf("invalid response type: %T", resp)
		}

		if !closeResp.Success {
			errMsg := closeResp.Error
			if errMsg == "" {
				errMsg = "bridge close failed"
			}
			return fmt.Errorf("%s", errMsg)
		}

		log.Printf("[NodeConn %s] Bridge %s closed successfully", nc.nodeID, bridgeID)
		return nil

	case <-timer.C:
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()
		return fmt.Errorf("timeout waiting for CloseBridge response")

	case <-ctx.Done():
		nc.pendingRequestsMu.Lock()
		delete(nc.pendingRequests, msgID)
		nc.pendingRequestsMu.Unlock()
		return ctx.Err()
	}
}

// SendData sends data through a bridge
func (nc *NodeConn) SendData(bridgeID string, data []byte) error {
	msgID := nc.getNextMessageID()

	msg := &shared.BridgeDataMessage{
		Message: shared.Message{
			Type: shared.MessageTypeBridgeData,
			ID:   msgID,
		},
		BridgeID: bridgeID,
		Data:     data,
	}

	return nc.transport.WriteMessage(msg)
}

// ============================================================
// Connection Management
// ============================================================

func (nc *NodeConn) Close() {
	nc.closeMu.Lock()
	defer nc.closeMu.Unlock()

	if nc.closed {
		return
	}
	nc.closed = true

	log.Printf("[NodeConn %s] Closing connection", nc.nodeID)

	close(nc.shutdown)

	// Close all bridge data channels
	nc.bridgeMu.Lock()
	for bridgeID, dataChan := range nc.bridgeChans {
		close(dataChan)
		log.Printf("[NodeConn %s] Closed bridge channel: %s", nc.nodeID, bridgeID)
	}
	nc.bridgeChans = make(map[string]chan []byte)
	nc.bridges = make(map[string]*Bridge)
	nc.bridgeMu.Unlock()

	// Clean up pending requests
	nc.pendingRequestsMu.Lock()
	for msgID, ch := range nc.pendingRequests {
		close(ch)
		log.Printf("[NodeConn %s] Closed pending request: %d", nc.nodeID, msgID)
	}
	nc.pendingRequests = make(map[uint64]chan any)
	nc.pendingRequestsMu.Unlock()

	if nc.server != nil && nc.nodeID != "" {
		nc.server.UnregisterNodeConnection(nc.nodeID)
		nc.server.nodeStore.RemoveNode(nc.nodeID)
		// Notify server that node is offline
		nc.server.notifyNodeOffline(nc.nodeID)
	}

	if nc.transport != nil {
		nc.transport.Close()
	}

	if nc.wsConn != nil {
		nc.wsConn.Close()
	}

	log.Printf("[NodeConn %s] Connection closed", nc.nodeID)
}

func (nc *NodeConn) NodeID() string {
	return nc.nodeID
}

// IsReady returns true if the node is ready for bridges
func (nc *NodeConn) IsReady() bool {
	return atomic.LoadInt32(&nc.ready) == 1
}

func (nc *NodeConn) isReady() bool {
	return atomic.LoadInt32(&nc.ready) == 1
}
