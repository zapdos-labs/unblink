package node_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/unblink/unblink/node"
)

// pipeConn creates a pair of connected net.Conn for testing
func pipeConn() (client, server net.Conn) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	done := make(chan struct{})
	go func() {
		server, _ = ln.Accept()
		close(done)
	}()
	client, _ = net.Dial("tcp", ln.Addr().String())
	<-done
	ln.Close()
	return
}

func TestConnSingleMessage(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	// Send from client
	msg := node.NewRegisterMsg("msg-001", "node-001")
	if err := clientConn.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// Receive on server
	received, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if received.MsgID != "msg-001" {
		t.Errorf("MsgID = %q, want %q", received.MsgID, "msg-001")
	}
	if received.Control.NodeID != "node-001" {
		t.Errorf("NodeID = %q, want %q", received.Control.NodeID, "node-001")
	}
}

func TestConnMultipleMessages(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	// Send multiple messages
	messages := []*node.Message{
		node.NewRegisterMsg("msg-1", "node-1"),
		node.NewAnnounceMsg("msg-2", []node.Service{{ID: "svc-1", Addr: "192.168.1.1", Port: 554}}),
		node.NewDataMsg("msg-3", "bridge-1", []byte("test payload")),
		node.NewAckMsg("msg-4", "msg-1"),
		node.NewCloseBridgeMsg("msg-5", "bridge-1"),
	}

	// Send all
	for _, msg := range messages {
		if err := clientConn.WriteMessage(msg); err != nil {
			t.Fatalf("WriteMessage failed: %v", err)
		}
	}

	// Receive all
	for i, expected := range messages {
		received, err := serverConn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage %d failed: %v", i, err)
		}
		if received.MsgID != expected.MsgID {
			t.Errorf("Message %d: MsgID = %q, want %q", i, received.MsgID, expected.MsgID)
		}
	}
}

func TestConnLargeMessage(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	// 1MB payload
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	msg := node.NewDataMsg("msg-large", "bridge-1", payload)
	if err := clientConn.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	received, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if !bytes.Equal(received.Data.Payload, payload) {
		t.Error("Large payload mismatch")
	}
}

func TestConnVeryLargeMessage(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	// 2MB payload (reasonable for testing)
	payload := make([]byte, 2*1024*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	msg := node.NewDataMsg("msg-verylarge", "bridge-1", payload)
	if err := clientConn.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	received, err := serverConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if !bytes.Equal(received.Data.Payload, payload) {
		t.Error("Very large payload mismatch")
	}
}

func TestConnBidirectional(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	var wg sync.WaitGroup
	wg.Add(2)

	// Client sends, server replies
	go func() {
		defer wg.Done()
		msg := node.NewRegisterMsg("msg-1", "node-1")
		if err := clientConn.WriteMessage(msg); err != nil {
			t.Errorf("Client WriteMessage failed: %v", err)
			return
		}

		reply, err := clientConn.ReadMessage()
		if err != nil {
			t.Errorf("Client ReadMessage failed: %v", err)
			return
		}
		if reply.ControlType() != node.MsgTypeAck {
			t.Errorf("Expected ACK, got %s", reply.ControlType())
		}
	}()

	go func() {
		defer wg.Done()
		msg, err := serverConn.ReadMessage()
		if err != nil {
			t.Errorf("Server ReadMessage failed: %v", err)
			return
		}

		reply := node.NewAckMsg("ack-1", msg.MsgID)
		if err := serverConn.WriteMessage(reply); err != nil {
			t.Errorf("Server WriteMessage failed: %v", err)
		}
	}()

	wg.Wait()
}

func TestConnConcurrentWrites(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	numMessages := 100
	var wg sync.WaitGroup

	// Start receiver
	received := make(chan string, numMessages)
	go func() {
		for i := 0; i < numMessages; i++ {
			msg, err := serverConn.ReadMessage()
			if err != nil {
				t.Errorf("ReadMessage %d failed: %v", i, err)
				return
			}
			received <- msg.MsgID
		}
		close(received)
	}()

	// Concurrent senders
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := node.NewDataMsg(
				string(rune('a'+i%26))+string(rune('0'+i/26)),
				"bridge-1",
				[]byte("test"),
			)
			if err := clientConn.WriteMessage(msg); err != nil {
				t.Errorf("WriteMessage %d failed: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all received
	count := 0
	for range received {
		count++
	}
	if count != numMessages {
		t.Errorf("Received %d messages, want %d", count, numMessages)
	}
}

func TestConnCloseWhileReading(t *testing.T) {
	client, server := pipeConn()
	defer server.Close()

	serverConn := node.NewConn(server)

	// Close client immediately
	client.Close()

	// Read should fail
	_, err := serverConn.ReadMessage()
	if err == nil {
		t.Error("Expected error on closed connection")
	}
}

func TestConnRawMessage(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	// Send raw bytes
	payload := []byte("raw CBOR bytes here")
	if err := clientConn.WriteRawMessage(payload); err != nil {
		t.Fatalf("WriteRawMessage failed: %v", err)
	}

	// Receive raw bytes
	received, err := serverConn.ReadRawMessage()
	if err != nil {
		t.Fatalf("ReadRawMessage failed: %v", err)
	}

	if !bytes.Equal(received, payload) {
		t.Errorf("Raw payload mismatch: %v != %v", received, payload)
	}
}

func TestConnFramingFormat(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)

	// Send a message
	msg := node.NewRegisterMsg("msg-001", "node-001")
	if err := clientConn.WriteMessage(msg); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	// Read raw bytes from server to verify framing
	// First 4 bytes should be big-endian length
	var lengthBuf [4]byte
	if _, err := io.ReadFull(server, lengthBuf[:]); err != nil {
		t.Fatalf("Read length failed: %v", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])
	if length == 0 || length > node.MaxMessageSize {
		t.Errorf("Invalid length: %d", length)
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(server, payload); err != nil {
		t.Fatalf("Read payload failed: %v", err)
	}

	// Verify it decodes correctly
	decoded, err := node.DecodeMessage(payload)
	if err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if decoded.MsgID != "msg-001" {
		t.Errorf("MsgID = %q, want %q", decoded.MsgID, "msg-001")
	}
}

func TestConnMessageTooLarge(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)

	// Try to send message larger than MaxMessageSize
	// This would require a 17MB+ payload which is expensive,
	// so instead we test the read side by sending a fake length

	// Write a fake length header > MaxMessageSize
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], node.MaxMessageSize+1)
	client.Write(lengthBuf[:])

	serverConn := node.NewConn(server)
	_, err := serverConn.ReadMessage()
	if err == nil {
		t.Error("Expected error for oversized message")
	}

	_ = clientConn // Silence unused warning
}

func TestConnEmptyMessage(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	// Write a zero-length message
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], 0)
	client.Write(lengthBuf[:])

	serverConn := node.NewConn(server)
	_, err := serverConn.ReadMessage()
	if err == nil {
		t.Error("Expected error for empty message")
	}
}

func TestConnTimeout(t *testing.T) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	serverConn := node.NewConn(server)

	// Set a short read deadline
	server.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

	// Try to read (no data coming)
	_, err := serverConn.ReadMessage()
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func BenchmarkConnWriteRead(b *testing.B) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	msg := node.NewDataMsg("msg-bench", "bridge-1", make([]byte, 1024))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.WriteMessage(msg)
		serverConn.ReadMessage()
	}
}

func BenchmarkConnLargeMessage(b *testing.B) {
	client, server := pipeConn()
	defer client.Close()
	defer server.Close()

	clientConn := node.NewConn(client)
	serverConn := node.NewConn(server)

	msg := node.NewDataMsg("msg-bench", "bridge-1", make([]byte, 64*1024)) // 64KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clientConn.WriteMessage(msg)
		serverConn.ReadMessage()
	}
}
