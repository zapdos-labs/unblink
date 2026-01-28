package node

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"unblink/shared"
)

// ConnState interface defines all possible operations
type ConnState interface {
	Name() string
	Connect(*Conn) error
	OnTokenCheckResponse(*Conn, *shared.TokenCheckResponse) error
	OnNewTokenResponse(*Conn, *shared.NewTokenResponse) error
	OnRegisterResponse(*Conn, *shared.RegisterResponse) error
	HandleOpenBridge(*Conn, *shared.OpenBridgeRequest) error
	HandleBridgeData(*Conn, *shared.BridgeDataMessage) error
	HandleCloseBridge(*Conn, *shared.CloseBridgeRequest) error
}

// baseState provides default error implementations
type baseState struct{ name string }

func (s *baseState) Name() string { return s.name }
func (s *baseState) Connect(*Conn) error {
	return fmt.Errorf("cannot connect in %s state", s.name)
}
func (s *baseState) OnTokenCheckResponse(*Conn, *shared.TokenCheckResponse) error {
	return fmt.Errorf("unexpected token_check_response in %s state", s.name)
}
func (s *baseState) OnNewTokenResponse(*Conn, *shared.NewTokenResponse) error {
	return fmt.Errorf("unexpected new_token_response in %s state", s.name)
}
func (s *baseState) OnRegisterResponse(*Conn, *shared.RegisterResponse) error {
	return fmt.Errorf("unexpected register_response in %s state", s.name)
}
func (s *baseState) HandleOpenBridge(*Conn, *shared.OpenBridgeRequest) error {
	return fmt.Errorf("cannot handle bridges in %s state", s.name)
}
func (s *baseState) HandleBridgeData(*Conn, *shared.BridgeDataMessage) error {
	return fmt.Errorf("cannot handle bridge data in %s state", s.name)
}
func (s *baseState) HandleCloseBridge(*Conn, *shared.CloseBridgeRequest) error {
	return fmt.Errorf("cannot handle bridge close in %s state", s.name)
}

// ============================================================
// DisconnectedState - initial state
// ============================================================

type DisconnectedState struct{ baseState }

func NewDisconnectedState() *DisconnectedState {
	return &DisconnectedState{baseState{name: "DISCONNECTED"}}
}

func (s *DisconnectedState) Connect(c *Conn) error {
	wsConn, _, err := websocket.DefaultDialer.Dial(c.configFile.Config.RelayAddress, nil)
	if err != nil {
		return err
	}

	c.wsConn = wsConn
	c.transport = &WebSocketConn{conn: wsConn}

	go c.messageLoop()

	// Check if we have an existing token
	if c.configFile.Config.Token != "" {
		c.setState(NewCheckingTokenState())
		return c.sendTokenCheckRequest()
	}

	// No token - request a new one
	c.setState(NewRequestingTokenState())
	return c.sendNewTokenRequest()
}

// ============================================================
// CheckingTokenState - verifying existing token
// ============================================================

type CheckingTokenState struct{ baseState }

func NewCheckingTokenState() *CheckingTokenState {
	return &CheckingTokenState{baseState{name: "CHECKING_TOKEN"}}
}

func (s *CheckingTokenState) OnTokenCheckResponse(c *Conn, r *shared.TokenCheckResponse) error {
	if r.Valid {
		log.Printf("[State] Token is valid, proceeding to register")
		c.setState(NewRegisteringState())
		return c.sendRegisterRequest()
	}

	// Token invalid - request a new one
	log.Printf("[State] Token invalid, requesting new token")
	c.setState(NewRequestingTokenState())
	return c.sendNewTokenRequest()
}

// ============================================================
// RequestingTokenState - waiting for new token
// ============================================================

type RequestingTokenState struct{ baseState }

func NewRequestingTokenState() *RequestingTokenState {
	return &RequestingTokenState{baseState{name: "REQUESTING_TOKEN"}}
}

func (s *RequestingTokenState) OnNewTokenResponse(c *Conn, r *shared.NewTokenResponse) error {
	if r.Error != "" {
		c.setState(NewClosedState())
		return fmt.Errorf("failed to get new token: %s", r.Error)
	}

	log.Printf("[State] Received new token, saving to config")

	// Save token to config
	c.configFile.Config.Token = r.Token
	if err := c.configFile.Save(); err != nil {
		log.Printf("[State] Warning: failed to save token to config: %v", err)
	}

	// Proceed to register
	c.setState(NewRegisteringState())
	return c.sendRegisterRequest()
}

// ============================================================
// RegisteringState - registering with validated token
// ============================================================

type RegisteringState struct{ baseState }

func NewRegisteringState() *RegisteringState {
	return &RegisteringState{baseState{name: "REGISTERING"}}
}

func (s *RegisteringState) OnRegisterResponse(c *Conn, r *shared.RegisterResponse) error {
	if !r.Success {
		c.setState(NewClosedState())
		return fmt.Errorf("registration failed: %s", r.Error)
	}

	log.Printf("[State] Registration successful, sending node_ready")

	if r.DashboardURL != "" {
		log.Printf("[Node] Dashboard URL: %s", r.DashboardURL)
	}

	c.setState(NewRegisteredState())

	msg := &shared.NodeReadyMessage{
		Message: shared.Message{
			Type: shared.MessageTypeNodeReady,
			ID:   c.getMsgID(),
		},
	}
	return c.transport.WriteMessage(msg)
}

// ============================================================
// RegisteredState - ready to handle bridges
// ============================================================

type RegisteredState struct{ baseState }

func NewRegisteredState() *RegisteredState {
	return &RegisteredState{baseState{name: "REGISTERED"}}
}

func (s *RegisteredState) HandleOpenBridge(c *Conn, r *shared.OpenBridgeRequest) error {
	return c.handleOpenBridge(r)
}

func (s *RegisteredState) HandleBridgeData(c *Conn, m *shared.BridgeDataMessage) error {
	return c.handleBridgeData(m)
}

func (s *RegisteredState) HandleCloseBridge(c *Conn, r *shared.CloseBridgeRequest) error {
	return c.handleCloseBridge(r)
}

// ============================================================
// ClosedState - terminal state
// ============================================================

type ClosedState struct{ baseState }

func NewClosedState() *ClosedState {
	return &ClosedState{baseState{name: "CLOSED"}}
}
