package node

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

// MaxMessageSize is the maximum allowed message size (16 MB)
const MaxMessageSize = 16 * 1024 * 1024

// MessageTransport defines the interface for reading/writing messages.
// Implemented by both Conn (TCP with length prefix) and WebSocketConn (WebSocket binary frames).
type MessageTransport interface {
	ReadMessage() (*Message, error)
	WriteMessage(*Message) error
	ReadRawMessage() ([]byte, error)
	WriteRawMessage([]byte) error
	Close() error
}

// Conn wraps a net.Conn with length-prefixed message framing.
// Message format: [4-byte big-endian length][CBOR payload]
type Conn struct {
	conn     net.Conn
	readMu   sync.Mutex
	writeMu  sync.Mutex
	readBuf  []byte // Reusable buffer for reading
}

// NewConn wraps a net.Conn with message framing
func NewConn(conn net.Conn) *Conn {
	return &Conn{
		conn:    conn,
		readBuf: make([]byte, 4096), // Initial buffer size
	}
}

// ReadMessage reads a length-prefixed message from the connection.
// Returns the decoded Message or an error.
func (c *Conn) ReadMessage() (*Message, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	// Read 4-byte length prefix
	var lengthBuf [4]byte
	if _, err := io.ReadFull(c.conn, lengthBuf[:]); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length == 0 {
		return nil, fmt.Errorf("empty message")
	}
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", length, MaxMessageSize)
	}

	// Grow buffer if needed
	if int(length) > len(c.readBuf) {
		c.readBuf = make([]byte, length)
	}

	// Read payload
	payload := c.readBuf[:length]
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	// Decode CBOR
	return DecodeMessage(payload)
}

// WriteMessage writes a length-prefixed message to the connection.
func (c *Conn) WriteMessage(msg *Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Encode to CBOR
	payload, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	if len(payload) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(payload), MaxMessageSize)
	}

	// Write length prefix
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(payload)))
	if _, err := c.conn.Write(lengthBuf[:]); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write payload
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// ReadRawMessage reads a length-prefixed raw message (without decoding).
// Useful for forwarding data without parsing.
func (c *Conn) ReadRawMessage() ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	// Read 4-byte length prefix
	var lengthBuf [4]byte
	if _, err := io.ReadFull(c.conn, lengthBuf[:]); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length == 0 {
		return nil, fmt.Errorf("empty message")
	}
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", length, MaxMessageSize)
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	return payload, nil
}

// WriteRawMessage writes a length-prefixed raw message (without encoding).
func (c *Conn) WriteRawMessage(payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if len(payload) > MaxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(payload), MaxMessageSize)
	}

	// Write length prefix
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(payload)))
	if _, err := c.conn.Write(lengthBuf[:]); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write payload
	if _, err := c.conn.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

// Close closes the underlying connection
func (c *Conn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// UnderlyingConn returns the underlying net.Conn
func (c *Conn) UnderlyingConn() net.Conn {
	return c.conn
}
