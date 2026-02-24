package node

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"unblink/shared"
)

// WebSocketConn wraps a WebSocket connection with the message interface
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

// ReadMessage reads a binary WebSocket message and decodes it
func (w *WebSocketConn) ReadMessage() (any, error) {
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

	// Decode CBOR
	return shared.DecodeMessage(data)
}

// WriteMessage encodes a message and writes it as a binary WebSocket message
func (w *WebSocketConn) WriteMessage(msg any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

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

// LocalAddr returns the local network address
func (w *WebSocketConn) LocalAddr() string {
	if w.conn != nil && w.conn.LocalAddr() != nil {
		return w.conn.LocalAddr().String()
	}
	return ""
}

// RemoteAddr returns the remote network address
func (w *WebSocketConn) RemoteAddr() string {
	if w.conn != nil && w.conn.RemoteAddr() != nil {
		return w.conn.RemoteAddr().String()
	}
	return ""
}

// UnderlyingConn returns the underlying WebSocket connection
func (w *WebSocketConn) UnderlyingConn() *websocket.Conn {
	return w.conn
}
