package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConn wraps a WebSocket connection with the same interface as Conn.
// Unlike Conn, it uses WebSocket's native binary message framing instead of length prefixes.
// Message format: [Binary WebSocket Frame containing CBOR payload]
type WebSocketConn struct {
	conn    *websocket.Conn
	readMu  sync.Mutex
	writeMu sync.Mutex
}

// NewWebSocketConn wraps a WebSocket connection
func NewWebSocketConn(conn *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{
		conn: conn,
	}
}

// ReadMessage reads a binary WebSocket message and decodes it.
// Returns the decoded Message or an error.
func (w *WebSocketConn) ReadMessage() (*Message, error) {
	w.readMu.Lock()
	defer w.readMu.Unlock()

	// Read WebSocket message
	messageType, data, err := w.conn.ReadMessage()
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

	// Decode CBOR directly (no length prefix needed)
	return DecodeMessage(data)
}

// WriteMessage encodes a message and writes it as a binary WebSocket message.
func (w *WebSocketConn) WriteMessage(msg *Message) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	// Encode to CBOR
	data, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	// Validate message size
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Write as binary WebSocket message
	if err := w.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write ws message: %w", err)
	}

	return nil
}

// ReadRawMessage reads a binary WebSocket message without decoding.
// Useful for forwarding data without parsing.
func (w *WebSocketConn) ReadRawMessage() ([]byte, error) {
	w.readMu.Lock()
	defer w.readMu.Unlock()

	// Read WebSocket message
	messageType, data, err := w.conn.ReadMessage()
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

	return data, nil
}

// WriteRawMessage writes raw data as a binary WebSocket message.
func (w *WebSocketConn) WriteRawMessage(data []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	// Validate message size
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(data), MaxMessageSize)
	}

	// Write as binary WebSocket message
	if err := w.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write ws message: %w", err)
	}

	return nil
}

// Close closes the WebSocket connection with a normal close message
func (w *WebSocketConn) Close() error {
	// Send close message with normal closure code
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = w.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	return w.conn.Close()
}

// SetPingHandler sets the handler for ping messages
func (w *WebSocketConn) SetPingHandler(h func(appData string) error) {
	w.conn.SetPingHandler(h)
}

// SetPongHandler sets the handler for pong messages
func (w *WebSocketConn) SetPongHandler(h func(appData string) error) {
	w.conn.SetPongHandler(h)
}

// SetReadDeadline sets the read deadline on the WebSocket connection
func (w *WebSocketConn) SetReadDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

// WriteControl writes a control message (ping/pong/close)
func (w *WebSocketConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return w.conn.WriteControl(messageType, data, deadline)
}

// UnderlyingConn returns the underlying WebSocket connection
func (w *WebSocketConn) UnderlyingConn() *websocket.Conn {
	return w.conn
}
