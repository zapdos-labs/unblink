package shared

import (
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// Message is the common header for all protocol messages
type Message struct {
	Type  string `json:"type"`
	ID    uint64 `json:"id"`
	Error string `json:"error,omitempty"`
}

// Message type constants
const (
	// Token messages
	MessageTypeTokenCheckRequest  = "token_check_request"
	MessageTypeTokenCheckResponse = "token_check_response"
	MessageTypeNewTokenRequest    = "new_token_request"
	MessageTypeNewTokenResponse   = "new_token_response"

	// Registration messages
	MessageTypeRegisterRequest  = "register_request"
	MessageTypeRegisterResponse = "register_response"
	MessageTypeNodeReady        = "node_ready"

	// Bridge messages
	MessageTypeOpenBridgeRequest   = "open_bridge_request"
	MessageTypeOpenBridgeResponse  = "open_bridge_response"
	MessageTypeCloseBridgeRequest  = "close_bridge_request"
	MessageTypeCloseBridgeResponse = "close_bridge_response"

	// Data messages
	MessageTypeBridgeData = "bridge_data"
)

// Messager interface for all message types
type Messager interface {
	GetType() string
	GetID() uint64
}

func (m *Message) GetType() string { return m.Type }
func (m *Message) GetID() uint64   { return m.ID }

// GetMessager extracts Messager interface from any message type
func GetMessager(msg any) (Messager, error) {
	if m, ok := msg.(Messager); ok {
		return m, nil
	}
	return nil, fmt.Errorf("message %T does not implement Messager interface", msg)
}

// ============================================================
// Token Messages
// ============================================================

// TokenCheckRequest - node checks if existing token is valid
type TokenCheckRequest struct {
	Message
	NodeID string `json:"node_id"`
	Token  string `json:"token"`
}

// TokenCheckResponse - server responds with validity
type TokenCheckResponse struct {
	Message
	Valid bool `json:"valid"`
}

// NewTokenRequest - node requests a new token
type NewTokenRequest struct {
	Message
	NodeID       string   `json:"node_id"`
	Hostname     string   `json:"hostname,omitempty"`
	MACAddresses []string `json:"mac_addresses,omitempty"`
}

// NewTokenResponse - server provides new token
type NewTokenResponse struct {
	Message
	Token string `json:"token"`
}

// ============================================================
// Registration Messages
// ============================================================

// RegisterRequest - node registers with validated token
type RegisterRequest struct {
	Message
	NodeID       string   `json:"node_id"`
	Token        string   `json:"token"`
	Hostname     string   `json:"hostname,omitempty"`
	MACAddresses []string `json:"mac_addresses,omitempty"`
}

// RegisterResponse - server confirms registration
type RegisterResponse struct {
	Message
	Success      bool   `json:"success"`
	Error        string `json:"error,omitempty"`
	DashboardURL string `json:"dashboard_url,omitempty"`
}

// NodeReadyMessage - node signals ready for bridges
type NodeReadyMessage struct {
	Message
}

// ============================================================
// Bridge Messages
// ============================================================

// Service represents a target service
type Service struct {
	ServiceURL string `json:"service_url"`
}

// OpenBridgeRequest - server requests node to open a bridge
type OpenBridgeRequest struct {
	Message
	BridgeID  string  `json:"bridge_id"`
	ServiceID string  `json:"service_id"`
	Service   Service `json:"service"`
}

// OpenBridgeResponse - node confirms bridge opened
type OpenBridgeResponse struct {
	Message
	Success bool `json:"success"`
}

// CloseBridgeRequest - server requests node to close a bridge
type CloseBridgeRequest struct {
	Message
	BridgeID string `json:"bridge_id"`
}

// CloseBridgeResponse - node confirms bridge closed
type CloseBridgeResponse struct {
	Message
	Success bool `json:"success"`
}

// BridgeDataMessage - bidirectional data for a bridge
type BridgeDataMessage struct {
	Message
	BridgeID string `json:"bridge_id"`
	Sequence uint64 `json:"sequence"`
	Data     []byte `json:"data"`
}

// ============================================================
// Encoding/Decoding
// ============================================================

func EncodeMessage(msg any) ([]byte, error) {
	return cbor.Marshal(msg)
}

func DecodeMessage(data []byte) (any, error) {
	var header Message
	if err := cbor.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode message header: %w", err)
	}

	typeMap := map[string]any{
		MessageTypeTokenCheckRequest:   &TokenCheckRequest{},
		MessageTypeTokenCheckResponse:  &TokenCheckResponse{},
		MessageTypeNewTokenRequest:     &NewTokenRequest{},
		MessageTypeNewTokenResponse:    &NewTokenResponse{},
		MessageTypeRegisterRequest:     &RegisterRequest{},
		MessageTypeRegisterResponse:    &RegisterResponse{},
		MessageTypeNodeReady:           &NodeReadyMessage{},
		MessageTypeOpenBridgeRequest:   &OpenBridgeRequest{},
		MessageTypeOpenBridgeResponse:  &OpenBridgeResponse{},
		MessageTypeCloseBridgeRequest:  &CloseBridgeRequest{},
		MessageTypeCloseBridgeResponse: &CloseBridgeResponse{},
		MessageTypeBridgeData:          &BridgeDataMessage{},
	}

	prototype, ok := typeMap[header.Type]
	if !ok {
		return nil, fmt.Errorf("unknown message type: %s", header.Type)
	}

	msgValue := reflect.New(reflect.TypeOf(prototype).Elem()).Interface()
	if err := cbor.Unmarshal(data, msgValue); err != nil {
		return nil, fmt.Errorf("decode %s: %w", header.Type, err)
	}

	return msgValue, nil
}
