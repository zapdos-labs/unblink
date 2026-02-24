package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"unblink/shared"
)

const MaxMessageSize = 16 * 1024 * 1024 // 16 MB

// MessageTransport defines the interface for reading/writing messages
type MessageTransport interface {
	ReadMessage() (any, error)
	WriteMessage(any) error
	Close() error
}

// wsMessageTransport wraps a WebSocket connection with CBOR message encoding/decoding
type wsMessageTransport struct {
	conn    *websocket.Conn
	readMu  sync.Mutex
	writeMu sync.Mutex
}

// NewMessageTransport creates a new message transport from a WebSocket connection
func NewMessageTransport(conn *websocket.Conn) *wsMessageTransport {
	return &wsMessageTransport{
		conn: conn,
	}
}

// ReadMessage reads a binary WebSocket message and decodes it
func (t *wsMessageTransport) ReadMessage() (any, error) {
	t.readMu.Lock()
	defer t.readMu.Unlock()

	// Read WebSocket message
	messageType, data, err := t.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read ws message: %w", err)
	}

	// Ensure it's a binary message
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("expected binary message, got type %d", messageType)
	}

	// Validate message size
	if len(data) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Decode CBOR
	return shared.DecodeMessage(data)
}

// WriteMessage encodes a message and writes it as a binary WebSocket message
func (t *wsMessageTransport) WriteMessage(msg any) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	// Encode to CBOR
	data, err := shared.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	// Validate message size
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Write as binary WebSocket message
	if err := t.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write ws message: %w", err)
	}

	return nil
}

// Close closes the WebSocket connection with a normal close message
func (t *wsMessageTransport) Close() error {
	// Send close message with normal closure code
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = t.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	return t.conn.Close()
}
