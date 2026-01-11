package node_test

import (
	"bytes"
	"testing"

	"github.com/unblink/unblink/node"
)

func TestRegisterMessage(t *testing.T) {
	msg := node.NewRegisterMsg("msg-001", "node-001")

	// Encode
	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify
	if decoded.MsgID != "msg-001" {
		t.Errorf("MsgID = %q, want %q", decoded.MsgID, "msg-001")
	}
	if !decoded.IsControl() {
		t.Error("Expected control message")
	}
	if decoded.ControlType() != node.MsgTypeRegister {
		t.Errorf("Type = %q, want %q", decoded.ControlType(), node.MsgTypeRegister)
	}
	if decoded.Control.NodeID != "node-001" {
		t.Errorf("NodeID = %q, want %q", decoded.Control.NodeID, "node-001")
	}
}

func TestAnnounceMessage(t *testing.T) {
	services := []node.Service{
		{ID: "cam1", Addr: "192.168.1.100", Port: 554, Type: "rtsp"},
		{ID: "cam2", Addr: "192.168.1.101", Port: 8080, Type: "mjpeg"},
	}
	msg := node.NewAnnounceMsg("msg-002", services)

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ControlType() != node.MsgTypeAnnounce {
		t.Errorf("Type = %q, want %q", decoded.ControlType(), node.MsgTypeAnnounce)
	}
	if len(decoded.Control.Services) != 2 {
		t.Errorf("Services count = %d, want 2", len(decoded.Control.Services))
	}
	if decoded.Control.Services[0].ID != "cam1" {
		t.Errorf("Service[0].ID = %q, want %q", decoded.Control.Services[0].ID, "cam1")
	}
	if decoded.Control.Services[1].Port != 8080 {
		t.Errorf("Service[1].Port = %d, want 8080", decoded.Control.Services[1].Port)
	}
}

func TestAckMessage(t *testing.T) {
	msg := node.NewAckMsg("ack-001", "msg-001")

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ControlType() != node.MsgTypeAck {
		t.Errorf("Type = %q, want %q", decoded.ControlType(), node.MsgTypeAck)
	}
	if decoded.Control.AckMsgID != "msg-001" {
		t.Errorf("AckMsgID = %q, want %q", decoded.Control.AckMsgID, "msg-001")
	}
}

func TestOpenBridgeMessage(t *testing.T) {
	service := &node.Service{ID: "cam1", Addr: "192.168.1.100", Port: 554, Type: "rtsp"}
	msg := node.NewOpenBridgeMsg("msg-003", "bridge-123", service)

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ControlType() != node.MsgTypeOpenBridge {
		t.Errorf("Type = %q, want %q", decoded.ControlType(), node.MsgTypeOpenBridge)
	}
	if decoded.Control.BridgeID != "bridge-123" {
		t.Errorf("BridgeID = %q, want %q", decoded.Control.BridgeID, "bridge-123")
	}
	if decoded.Control.Service == nil {
		t.Fatal("Service is nil")
	}
	if decoded.Control.Service.Addr != "192.168.1.100" {
		t.Errorf("Service.Addr = %q, want %q", decoded.Control.Service.Addr, "192.168.1.100")
	}
}

func TestCloseBridgeMessage(t *testing.T) {
	msg := node.NewCloseBridgeMsg("msg-004", "bridge-123")

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ControlType() != node.MsgTypeCloseBridge {
		t.Errorf("Type = %q, want %q", decoded.ControlType(), node.MsgTypeCloseBridge)
	}
	if decoded.Control.BridgeID != "bridge-123" {
		t.Errorf("BridgeID = %q, want %q", decoded.Control.BridgeID, "bridge-123")
	}
}

func TestDataMessage(t *testing.T) {
	payload := []byte("Hello, World!")
	msg := node.NewDataMsg("msg-005", "bridge-123", payload)

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.IsControl() {
		t.Error("Expected data message, got control")
	}
	if !decoded.IsData() {
		t.Error("Expected data message")
	}
	if decoded.Data.BridgeID != "bridge-123" {
		t.Errorf("BridgeID = %q, want %q", decoded.Data.BridgeID, "bridge-123")
	}
	if !bytes.Equal(decoded.Data.Payload, payload) {
		t.Errorf("Payload = %v, want %v", decoded.Data.Payload, payload)
	}
}

func TestLargePayload(t *testing.T) {
	// Test with 1MB payload
	payload := make([]byte, 1024*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	msg := node.NewDataMsg("msg-large", "bridge-123", payload)

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(decoded.Data.Payload, payload) {
		t.Error("Large payload mismatch")
	}
}

func TestEmptyServices(t *testing.T) {
	msg := node.NewAnnounceMsg("msg-empty", []node.Service{})

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(decoded.Control.Services) != 0 {
		t.Errorf("Services count = %d, want 0", len(decoded.Control.Services))
	}
}

func TestEmptyPayload(t *testing.T) {
	msg := node.NewDataMsg("msg-empty-data", "bridge-123", []byte{})

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(decoded.Data.Payload) != 0 {
		t.Errorf("Payload length = %d, want 0", len(decoded.Data.Payload))
	}
}

func TestMessageHelpers(t *testing.T) {
	// Test IsControl/IsData
	controlMsg := node.NewRegisterMsg("msg-1", "node-1")
	if !controlMsg.IsControl() {
		t.Error("Register should be control")
	}
	if controlMsg.IsData() {
		t.Error("Register should not be data")
	}

	dataMsg := node.NewDataMsg("msg-2", "bridge-1", []byte("test"))
	if dataMsg.IsControl() {
		t.Error("Data should not be control")
	}
	if !dataMsg.IsData() {
		t.Error("Data should be data")
	}

	// Test ControlType
	if controlMsg.ControlType() != node.MsgTypeRegister {
		t.Errorf("ControlType = %q, want %q", controlMsg.ControlType(), node.MsgTypeRegister)
	}
	if dataMsg.ControlType() != "" {
		t.Errorf("Data ControlType = %q, want empty", dataMsg.ControlType())
	}
}

func TestLoadAllowList(t *testing.T) {
	allowList, err := node.LoadAllowList("../allowlist.json")
	if err != nil {
		t.Fatalf("LoadAllowList failed: %v", err)
	}

	if len(allowList.Services) != 2 {
		t.Fatalf("Expected 2 services, got %d", len(allowList.Services))
	}

	// Check rtsp-camera service
	rtsp := allowList.Services[0]
	if rtsp.ID != "rtsp-camera" {
		t.Errorf("Service[0].ID = %q, want %q", rtsp.ID, "rtsp-camera")
	}
	if rtsp.Type != "rtsp" {
		t.Errorf("Service[0].Type = %q, want %q", rtsp.Type, "rtsp")
	}
	if rtsp.Addr != "192.168.178.65" {
		t.Errorf("Service[0].Addr = %q, want %q", rtsp.Addr, "192.168.178.65")
	}
	if rtsp.Port != 554 {
		t.Errorf("Service[0].Port = %d, want 554", rtsp.Port)
	}
	if rtsp.Path != "/stream2" {
		t.Errorf("Service[0].Path = %q, want %q", rtsp.Path, "/stream2")
	}
	if rtsp.Auth == nil {
		t.Fatal("Service[0].Auth is nil")
	}
	if rtsp.Auth.Type != node.AuthTypeUsernamePassword {
		t.Errorf("Service[0].Auth.Type = %q, want %q", rtsp.Auth.Type, node.AuthTypeUsernamePassword)
	}
	if rtsp.Auth.Username != "vantri" {
		t.Errorf("Service[0].Auth.Username = %q, want %q", rtsp.Auth.Username, "vantri")
	}

	// Check http-dashboard service (no auth)
	http := allowList.Services[1]
	if http.ID != "http-dashboard" {
		t.Errorf("Service[1].ID = %q, want %q", http.ID, "http-dashboard")
	}
	if http.Auth != nil {
		t.Errorf("Service[1].Auth should be nil, got %+v", http.Auth)
	}
}

func TestServiceWithPathAndAuth(t *testing.T) {
	// Test encoding/decoding service with all fields
	service := node.Service{
		ID:   "test-rtsp",
		Type: "rtsp",
		Addr: "192.168.1.100",
		Port: 554,
		Path: "/live/stream1",
		Auth: &node.Auth{
			Type:     node.AuthTypeUsernamePassword,
			Username: "admin",
			Password: "secret",
		},
	}

	msg := node.NewOpenBridgeMsg("msg-auth", "bridge-auth", &service)

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := node.DecodeMessage(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	svc := decoded.Control.Service
	if svc.Path != "/live/stream1" {
		t.Errorf("Path = %q, want %q", svc.Path, "/live/stream1")
	}
	if svc.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if svc.Auth.Type != node.AuthTypeUsernamePassword {
		t.Errorf("Auth.Type = %q, want %q", svc.Auth.Type, node.AuthTypeUsernamePassword)
	}
	if svc.Auth.Username != "admin" {
		t.Errorf("Auth.Username = %q, want %q", svc.Auth.Username, "admin")
	}
	if svc.Auth.Password != "secret" {
		t.Errorf("Auth.Password = %q, want %q", svc.Auth.Password, "secret")
	}
}

func BenchmarkEncode(b *testing.B) {
	msg := node.NewDataMsg("msg-bench", "bridge-123", make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.Encode()
	}
}

func BenchmarkDecode(b *testing.B) {
	msg := node.NewDataMsg("msg-bench", "bridge-123", make([]byte, 1024))
	data, _ := msg.Encode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = node.DecodeMessage(data)
	}
}
