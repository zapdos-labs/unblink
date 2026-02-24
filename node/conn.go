package node

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"unblink/shared"
)

const MaxMessageSize = 16 * 1024 * 1024 // 16 MB

// MessageTransport defines the interface for reading/writing messages
type MessageTransport interface {
	ReadMessage() (any, error)
	WriteMessage(any) error
	Close() error
}

// Conn manages the connection to the server
type Conn struct {
	configFile *ConfigFile // Config with its path
	wsConn     any         // *websocket.Conn

	// Transport
	transport MessageTransport

	// State management
	state   ConnState
	stateMu sync.RWMutex

	// Node state
	nodeID   string
	shutdown chan struct{}
	closed   bool
	closeMu  sync.Mutex

	// Bridges
	bridges  map[string]*Bridge
	bridgeMu sync.RWMutex

	// Message ID counter
	msgID uint64
	msgMu sync.Mutex
}

// Bridge represents an active bridge from node to service
type Bridge struct {
	ID        string
	ServiceID string
	Addr      string
	Port      int
	Path      string
	Conn      net.Conn
	shutdown  chan struct{}
	byteCount int64
	msgCount  int64
	closeOnce sync.Once
}

// NewConn creates a new node connection
func NewConn(configFile *ConfigFile) *Conn {
	c := &Conn{
		configFile: configFile,
		shutdown:   make(chan struct{}),
		bridges:    make(map[string]*Bridge),
		msgID:      1,
	}
	c.state = NewDisconnectedState()
	return c
}

// setState atomically transitions to a new state
func (c *Conn) setState(newState ConnState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	log.Printf("[State] %s -> %s", c.state.Name(), newState.Name())
	c.state = newState
}

// Connect establishes a connection to the server
func (c *Conn) Connect() error {
	c.stateMu.RLock()
	state := c.state
	c.stateMu.RUnlock()

	return state.Connect(c)
}

// Run connects to the server and blocks until disconnection
func (c *Conn) Run() error {
	// Connect using state machine
	if err := c.Connect(); err != nil {
		return err
	}

	// Wait for message loop to exit (disconnection or shutdown)
	<-c.shutdown

	return nil
}

// messageLoop reads messages from server and dispatches them to handlers
func (c *Conn) messageLoop() {
	defer func() {
		log.Printf("[Conn] Message loop ended")
		// Signal that message loop has ended
		select {
		case <-c.shutdown:
			// Already closed
		default:
			close(c.shutdown)
		}
	}()

	for {
		select {
		case <-c.shutdown:
			return
		default:
		}

		msg, err := c.transport.ReadMessage()
		if err != nil {
			log.Printf("[Conn] Read error: %v", err)
			return
		}

		if err := c.handleMessage(msg); err != nil {
			log.Printf("[Conn] Error handling message: %v", err)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler based on its type
func (c *Conn) handleMessage(msg any) error {
	m, err := shared.GetMessager(msg)
	if err != nil {
		return fmt.Errorf("invalid message type: %w", err)
	}

	msgType := m.GetType()

	c.stateMu.RLock()
	state := c.state
	c.stateMu.RUnlock()

	switch msgType {
	case shared.MessageTypeTokenCheckResponse:
		return state.OnTokenCheckResponse(c, msg.(*shared.TokenCheckResponse))

	case shared.MessageTypeNewTokenResponse:
		return state.OnNewTokenResponse(c, msg.(*shared.NewTokenResponse))

	case shared.MessageTypeRegisterResponse:
		return state.OnRegisterResponse(c, msg.(*shared.RegisterResponse))

	case shared.MessageTypeOpenBridgeRequest:
		return state.HandleOpenBridge(c, msg.(*shared.OpenBridgeRequest))

	case shared.MessageTypeCloseBridgeRequest:
		return state.HandleCloseBridge(c, msg.(*shared.CloseBridgeRequest))

	case shared.MessageTypeBridgeData:
		return state.HandleBridgeData(c, msg.(*shared.BridgeDataMessage))

	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

// ============================================================
// Token & Registration Messages
// ============================================================

// sendTokenCheckRequest sends a token check request to the server
func (c *Conn) sendTokenCheckRequest() error {
	msg := &shared.TokenCheckRequest{
		Message: shared.Message{
			Type: shared.MessageTypeTokenCheckRequest,
			ID:   c.getMsgID(),
		},
		NodeID: c.configFile.Config.NodeID,
		Token:  c.configFile.Config.Token,
	}

	log.Printf("[Conn] Sending token_check_request: nodeID=%s", c.configFile.Config.NodeID)
	return c.transport.WriteMessage(msg)
}

// sendNewTokenRequest sends a new token request to the server
func (c *Conn) sendNewTokenRequest() error {
	hostname, macs, _ := getSystemInfo()

	msg := &shared.NewTokenRequest{
		Message: shared.Message{
			Type: shared.MessageTypeNewTokenRequest,
			ID:   c.getMsgID(),
		},
		NodeID:       c.configFile.Config.NodeID,
		Hostname:     hostname,
		MACAddresses: macs,
	}

	log.Printf("[Conn] Sending new_token_request: nodeID=%s", c.configFile.Config.NodeID)
	return c.transport.WriteMessage(msg)
}

// sendRegisterRequest sends a register request to the server
func (c *Conn) sendRegisterRequest() error {
	hostname, macs, _ := getSystemInfo()

	msg := &shared.RegisterRequest{
		Message: shared.Message{
			Type: shared.MessageTypeRegisterRequest,
			ID:   c.getMsgID(),
		},
		NodeID:       c.configFile.Config.NodeID,
		Token:        c.configFile.Config.Token,
		Hostname:     hostname,
		MACAddresses: macs,
	}

	log.Printf("[Conn] Sending register_request: nodeID=%s", c.configFile.Config.NodeID)
	return c.transport.WriteMessage(msg)
}

// ============================================================
// Bridge Handlers
// ============================================================

func (c *Conn) handleOpenBridge(req *shared.OpenBridgeRequest) error {
	parsed, err := shared.ParseServiceURL(req.Service.ServiceURL)
	if err != nil {
		log.Printf("[Conn] Failed to parse service URL %s: %v", req.Service.ServiceURL, err)
		resp := &shared.OpenBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeOpenBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("parse service URL: %w", err)
	}

	log.Printf("[Conn] OpenBridge request: bridgeID=%s, service=%s:%d%s",
		req.BridgeID, parsed.Host, parsed.Port, parsed.Path)

	serviceAddr := net.JoinHostPort(parsed.Host, strconv.Itoa(parsed.Port))
	svcConn, err := net.DialTimeout("tcp", serviceAddr, 10*time.Second)
	if err != nil {
		log.Printf("[Conn] Failed to connect to service %s: %v", serviceAddr, err)
		resp := &shared.OpenBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeOpenBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("connect to service: %w", err)
	}

	log.Printf("[Conn] Connected to service")

	bridge := &Bridge{
		ID:        req.BridgeID,
		ServiceID: req.ServiceID,
		Addr:      parsed.Host,
		Port:      parsed.Port,
		Path:      parsed.Path,
		Conn:      svcConn,
		shutdown:  make(chan struct{}),
	}

	c.bridgeMu.Lock()
	c.bridges[req.BridgeID] = bridge
	c.bridgeMu.Unlock()

	resp := &shared.OpenBridgeResponse{
		Message: shared.Message{
			Type: shared.MessageTypeOpenBridgeResponse,
			ID:   req.ID,
		},
		Success: true,
	}

	if err := c.transport.WriteMessage(resp); err != nil {
		c.closeBridge(bridge)
		return fmt.Errorf("send open bridge response: %w", err)
	}

	log.Printf("[Conn] Opened bridge %s to %s:%d%s", req.BridgeID, parsed.Host, parsed.Port, parsed.Path)

	go c.forwardServiceToServer(bridge)

	return nil
}

func (c *Conn) handleCloseBridge(req *shared.CloseBridgeRequest) error {
	log.Printf("[Conn] CloseBridge request: bridgeID=%s", req.BridgeID)

	c.bridgeMu.RLock()
	bridge, exists := c.bridges[req.BridgeID]
	c.bridgeMu.RUnlock()

	if !exists {
		resp := &shared.CloseBridgeResponse{
			Message: shared.Message{
				Type: shared.MessageTypeCloseBridgeResponse,
				ID:   req.ID,
			},
			Success: false,
		}
		_ = c.transport.WriteMessage(resp)
		return fmt.Errorf("bridge not found: %s", req.BridgeID)
	}

	c.closeBridge(bridge)

	resp := &shared.CloseBridgeResponse{
		Message: shared.Message{
			Type: shared.MessageTypeCloseBridgeResponse,
			ID:   req.ID,
		},
		Success: true,
	}

	if err := c.transport.WriteMessage(resp); err != nil {
		return fmt.Errorf("send close bridge response: %w", err)
	}

	log.Printf("[Conn] Closed bridge %s", req.BridgeID)
	return nil
}

func (c *Conn) handleBridgeData(msg *shared.BridgeDataMessage) error {
	c.bridgeMu.RLock()
	bridge, exists := c.bridges[msg.BridgeID]
	c.bridgeMu.RUnlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", msg.BridgeID)
	}

	_, err := bridge.Conn.Write(msg.Data)
	if err != nil {
		log.Printf("[Conn] Write to service failed: %v", err)
		c.closeBridge(bridge)
		return fmt.Errorf("write to service: %w", err)
	}

	atomic.AddInt64(&bridge.byteCount, int64(len(msg.Data)))
	atomic.AddInt64(&bridge.msgCount, 1)

	return nil
}

func (c *Conn) forwardServiceToServer(bridge *Bridge) {
	defer log.Printf("[Conn] Stopped forwarding for bridge %s", bridge.ID)

	buf := make([]byte, 4096)
	var sequence uint64

	log.Printf("[Conn] Started forwarding for bridge %s", bridge.ID)

	for {
		select {
		case <-bridge.shutdown:
			log.Printf("[Conn] Shutdown signal received for bridge %s", bridge.ID)
			return
		default:
			bridge.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, err := bridge.Conn.Read(buf)

			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if err != io.EOF {
					log.Printf("[Conn] Read from service error: %v", err)
				}
				return
			}

			if n > 0 {
				sequence++

				msg := &shared.BridgeDataMessage{
					Message: shared.Message{
						Type: shared.MessageTypeBridgeData,
						ID:   c.getMsgID(),
					},
					BridgeID: bridge.ID,
					Sequence: sequence,
					Data:     buf[:n],
				}

				if err := c.transport.WriteMessage(msg); err != nil {
					log.Printf("[Conn] Failed to send data to server: %v", err)
					return
				}
			}
		}
	}
}

func (c *Conn) closeBridge(bridge *Bridge) {
	bridge.closeOnce.Do(func() {
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
		}

		c.bridgeMu.Lock()
		delete(c.bridges, bridge.ID)
		c.bridgeMu.Unlock()
	})
}

// ============================================================
// Utility Methods
// ============================================================

func (c *Conn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	close(c.shutdown)

	c.bridgeMu.Lock()
	for _, bridge := range c.bridges {
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
		}
	}
	c.bridges = make(map[string]*Bridge)
	c.bridgeMu.Unlock()

	if c.transport != nil {
		c.transport.Close()
	}

	return nil
}

func (c *Conn) getMsgID() uint64 {
	c.msgMu.Lock()
	defer c.msgMu.Unlock()

	id := c.msgID
	c.msgID++
	return id
}

func (c *Conn) NodeID() string {
	return c.configFile.Config.NodeID
}

func getSystemInfo() (string, []string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("[Conn] Failed to get hostname: %v", err)
		hostname = "unknown"
	}

	var macs []string
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			if len(iface.HardwareAddr) > 0 {
				macs = append(macs, iface.HardwareAddr.String())
			}
		}
	} else {
		log.Printf("[Conn] Failed to get network interfaces: %v", err)
	}

	log.Printf("[Conn] System info: hostname=%q macs=%v", hostname, macs)
	return hostname, macs, nil
}

func (c *Conn) CloseBridge(bridgeID string) error {
	c.bridgeMu.Lock()
	bridge, exists := c.bridges[bridgeID]
	if exists {
		delete(c.bridges, bridgeID)
	}
	c.bridgeMu.Unlock()

	if !exists {
		return fmt.Errorf("bridge not found: %s", bridgeID)
	}

	if bridge.Conn != nil {
		close(bridge.shutdown)
		bridge.Conn.Close()
		log.Printf("[Conn] Closed bridge %s", bridgeID)
	}

	return nil
}

func (c *Conn) CloseAllBridges() error {
	c.bridgeMu.Lock()
	defer c.bridgeMu.Unlock()

	for _, bridge := range c.bridges {
		if bridge.Conn != nil {
			close(bridge.shutdown)
			bridge.Conn.Close()
			log.Printf("[Conn] Closed bridge %s", bridge.ID)
		}
	}

	c.bridges = make(map[string]*Bridge)
	return nil
}
