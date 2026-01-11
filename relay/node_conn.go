package relay

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/unblink/unblink/node"
)

// Bridge represents an active data bridge
type Bridge struct {
	ID        string
	Service   node.Service
	ByteCount int64
	MsgCount  int64
}

// NodeConn handles a single node connection
type NodeConn struct {
	conn        *node.Conn
	relay       *Relay
	nodeID      string
	authToken   string // Authorization token for this node
	bridges     map[string]*Bridge
	bridgeMu    sync.RWMutex
	bridgeChans map[string]chan []byte // bridgeID → data channel
	shutdown    chan struct{}
	closed      bool
	closeMu     sync.Mutex
}

// Run handles incoming messages from the node
func (nc *NodeConn) Run() error {
	for {
		select {
		case <-nc.shutdown:
			return nil
		default:
		}

		msg, err := nc.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		if err := nc.handleMessage(msg); err != nil {
			log.Printf("[NodeConn] Error handling message: %v", err)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler
func (nc *NodeConn) handleMessage(msg *node.Message) error {
	if msg.IsControl() {
		return nc.handleControl(msg)
	}
	if msg.IsData() {
		return nc.handleData(msg)
	}
	return fmt.Errorf("unknown message type")
}

// handleControl handles control messages
func (nc *NodeConn) handleControl(msg *node.Message) error {
	ctrl := msg.Control

	switch ctrl.Type {
	case node.MsgTypeRegister:
		return nc.handleRegister(msg.MsgID, ctrl.NodeID, ctrl.Token)

	case node.MsgTypeReqAuthorizationURL:
		return nc.handleReqAuthorizationURL(msg.MsgID, ctrl.NodeID)

	case node.MsgTypeAnnounce:
		return nc.handleAnnounce(msg.MsgID, ctrl.Services)

	case node.MsgTypeAck:
		// Node acknowledging our message - log and ignore for now
		log.Printf("[NodeConn %s] Received ACK for %s", nc.nodeID, ctrl.AckMsgID)
		return nil

	default:
		return fmt.Errorf("unknown control type: %s", ctrl.Type)
	}
}

// handleRegister handles REGISTER messages
func (nc *NodeConn) handleRegister(msgID, clientNodeID, token string) error {
	// If token provided, validate it
	if token != "" {
		nodeData, err := nc.relay.nodeTable.GetNodeByToken(token)
		if err != nil {
			log.Printf("[NodeConn] Invalid token: %v", err)
			sendErr := nc.sendRegisterError(msgID, "invalid_token")
			if sendErr != nil {
				log.Printf("[NodeConn] Failed to send register error: %v", sendErr)
			}
			return fmt.Errorf("invalid authorization token")
		}

		// Check if node is authorized
		if nodeData.OwnerID == nil {
			sendErr := nc.sendRegisterError(msgID, "unauthorized")
			if sendErr != nil {
				log.Printf("[NodeConn] Failed to send register error: %v", sendErr)
			}
			return fmt.Errorf("node not yet authorized")
		}

		// Token is valid and node is authorized, use this node ID
		nodeID := nodeData.ID
		nc.authToken = token

		nc.relay.registerNode(nodeID, nc)

		// Update last connected
		nc.relay.nodeTable.UpdateLastConnected(nodeID)

		// Send ACK
		if err := nc.sendAck(msgID); err != nil {
			return err
		}

		// Send connection_ready WITHOUT dashboard URL
		return nc.sendConnectionReady(nodeID, "")
	}

	// No token - reject (should use authorization flow)
	sendErr := nc.sendRegisterError(msgID, "missing_token")
	if sendErr != nil {
		log.Printf("[NodeConn] Failed to send register error: %v", sendErr)
	}
	return fmt.Errorf("no token provided, please use authorization flow")
}

// handleReqAuthorizationURL handles REQ_AUTHORIZATION_URL messages
func (nc *NodeConn) handleReqAuthorizationURL(msgID, nodeID string) error {
	// Store the node ID for later use when authorization completes
	nc.nodeID = nodeID

	// Register this connection in the nodes map so we can find it later
	nc.relay.nodesMu.Lock()
	nc.relay.nodes[nodeID] = nc
	nc.relay.nodesMu.Unlock()

	// Generate authorization URL
	authURL := fmt.Sprintf("%s/authorize?node=%s", nc.relay.config.DashboardURL, nodeID)

	// Send ACK
	if err := nc.sendAck(msgID); err != nil {
		return err
	}

	// Send RES_AUTHORIZATION_URL with the auth URL
	msg := node.NewResAuthorizationURLMsg(uuid.New().String(), authURL)
	if err := nc.conn.WriteMessage(msg); err != nil {
		return fmt.Errorf("send res authorization URL: %w", err)
	}

	log.Printf("[NodeConn] Node %s requested authorization, sent URL", nodeID)
	return nil
}

// sendConnectionReady sends a CONNECTION_READY message
func (nc *NodeConn) sendConnectionReady(nodeID, dashboardURL string) error {
	msg := node.NewConnectionReadyMsg(uuid.New().String(), nodeID, dashboardURL)
	return nc.conn.WriteMessage(msg)
}

// sendRegisterError sends a REGISTER_ERROR message to the node
func (nc *NodeConn) sendRegisterError(msgID, errorCode string) error {
	errorMsg := registerErrorMessages[errorCode]
	if errorMsg == "" {
		errorMsg = "Unknown registration error"
	}

	msg := node.NewRegisterErrorMsg(msgID, errorCode, errorMsg)
	if err := nc.conn.WriteMessage(msg); err != nil {
		return fmt.Errorf("send register error: %w", err)
	}
	log.Printf("[NodeConn] Sent register error to node: code=%s, msg=%s", errorCode, errorMsg)
	return nil
}

// registerErrorMessages maps error codes to human-readable messages
var registerErrorMessages = map[string]string{
	"invalid_token":  "Invalid authorization token",
	"unauthorized":   "Node not yet authorized. Please complete authorization flow.",
	"missing_token":  "No token provided. Please use authorization flow.",
	"not_registered": "Node not registered. Complete registration first.",
}

// handleAnnounce handles ANNOUNCE messages
func (nc *NodeConn) handleAnnounce(msgID string, services []node.Service) error {
	if nc.nodeID == "" {
		sendErr := nc.sendRegisterError(msgID, "not_registered")
		if sendErr != nil {
			log.Printf("[NodeConn] Failed to send register error: %v", sendErr)
		}
		return fmt.Errorf("node not registered")
	}

	// Register services
	for _, svc := range services {
		nc.relay.services.Register(svc, nc.nodeID)
		log.Printf("[NodeConn %s] Service announced: %s (%s:%d)",
			nc.nodeID, svc.ID, svc.Addr, svc.Port)

		// Start realtime stream for camera services (if enabled)
		// IMPORTANT: Run async to avoid blocking the message loop (would cause deadlock)
		if nc.relay.config.AutoRequestRealtimeStream && nc.relay.realtimeStreamManager != nil && (svc.Type == "rtsp" || svc.Type == "mjpeg") {
			go nc.relay.realtimeStreamManager.OnServiceAnnounced(svc, nc.nodeID)
		}
	}

	// Send ACK
	return nc.sendAck(msgID)
}

// handleData handles DATA messages
func (nc *NodeConn) handleData(msg *node.Message) error {
	data := msg.Data

	nc.bridgeMu.Lock()
	bridge, exists := nc.bridges[data.BridgeID]
	if !exists {
		nc.bridgeMu.Unlock()
		return fmt.Errorf("unknown bridge: %s", data.BridgeID)
	}

	// Update statistics
	bridge.MsgCount++
	bridge.ByteCount += int64(len(data.Payload))
	nc.bridgeMu.Unlock()

	// Forward data to registered consumers
	nc.bridgeMu.RLock()
	ch, exists := nc.bridgeChans[data.BridgeID]
	nc.bridgeMu.RUnlock()

	if exists {
		select {
		case ch <- data.Payload:
		default:
			// Channel full or closed, drop packet
			log.Printf("[NodeConn %s] Bridge %s: channel full, dropping packet", nc.nodeID, data.BridgeID)
		}
	}

	return nil
}

// sendAck sends an ACK message
func (nc *NodeConn) sendAck(ackMsgID string) error {
	ack := node.NewAckMsg(uuid.New().String(), ackMsgID)
	return nc.conn.WriteMessage(ack)
}

// OpenBridge requests the node to open a bridge to a service
func (nc *NodeConn) OpenBridge(service node.Service) (string, error) {
	bridgeID := uuid.New().String()
	msgID := uuid.New().String()

	msg := node.NewOpenBridgeMsg(msgID, bridgeID, &service)
	if err := nc.conn.WriteMessage(msg); err != nil {
		return "", fmt.Errorf("send open_bridge: %w", err)
	}

	// Store bridge
	nc.bridgeMu.Lock()
	nc.bridges[bridgeID] = &Bridge{
		ID:      bridgeID,
		Service: service,
	}
	nc.bridgeMu.Unlock()

	log.Printf("[NodeConn %s] Opened bridge %s to %s:%d",
		nc.nodeID, bridgeID, service.Addr, service.Port)

	return bridgeID, nil
}

// CloseBridge requests the node to close a bridge
func (nc *NodeConn) CloseBridge(bridgeID string) error {
	nc.bridgeMu.Lock()
	delete(nc.bridges, bridgeID)
	nc.bridgeMu.Unlock()

	msgID := uuid.New().String()
	msg := node.NewCloseBridgeMsg(msgID, bridgeID)
	if err := nc.conn.WriteMessage(msg); err != nil {
		return fmt.Errorf("send close_bridge: %w", err)
	}

	log.Printf("[NodeConn %s] Closed bridge %s", nc.nodeID, bridgeID)
	return nil
}

// SendData sends data over a bridge
func (nc *NodeConn) SendData(bridgeID string, payload []byte) error {
	msgID := uuid.New().String()
	msg := node.NewDataMsg(msgID, bridgeID, payload)
	return nc.conn.WriteMessage(msg)
}

// Close closes the node connection and all bridges
func (nc *NodeConn) Close() {
	nc.closeMu.Lock()
	defer nc.closeMu.Unlock()

	if nc.closed {
		return
	}
	nc.closed = true

	close(nc.shutdown)

	// Close all bridge channels to signal consumers (like BridgeTCPProxy) to stop
	nc.bridgeMu.Lock()
	for bridgeID, ch := range nc.bridgeChans {
		close(ch)
		log.Printf("[NodeConn %s] Closed bridge channel %s", nc.nodeID, bridgeID)
	}
	nc.bridgeChans = make(map[string]chan []byte)
	nc.bridges = make(map[string]*Bridge)
	nc.bridgeMu.Unlock()

	nc.conn.Close()
}

// NodeID returns the node's ID
func (nc *NodeConn) NodeID() string {
	return nc.nodeID
}

// RegisterBridgeChan registers a channel to receive data for a bridge
func (nc *NodeConn) RegisterBridgeChan(bridgeID string, ch chan []byte) {
	nc.bridgeMu.Lock()
	defer nc.bridgeMu.Unlock()
	nc.bridgeChans[bridgeID] = ch
}

// UnregisterBridgeChan removes the data channel for a bridge
func (nc *NodeConn) UnregisterBridgeChan(bridgeID string) {
	nc.bridgeMu.Lock()
	defer nc.bridgeMu.Unlock()
	if ch, exists := nc.bridgeChans[bridgeID]; exists {
		close(ch)
		delete(nc.bridgeChans, bridgeID)
	}
}
