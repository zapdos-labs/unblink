package webrtc

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"

	"unblink/server"
	webrtcv1 "unblink/server/gen/webrtc/v1"
)

// Service implements the WebRTC gRPC/Connect service
type Service struct {
	server     *server.Server
	sessionMgr *SessionManager
}

// NewService creates a new WebRTC service
func NewService(srv *server.Server) *Service {
	return &Service{
		server:     srv,
		sessionMgr: NewSessionManager(),
	}
}

// CreateWebRTCSession implements the CreateWebRTCSession RPC
func (s *Service) CreateWebRTCSession(
	ctx context.Context,
	req *connect.Request[webrtcv1.CreateWebRTCSessionRequest],
) (*connect.Response[webrtcv1.CreateWebRTCSessionResponse], error) {
	log.Printf("[WebRTC Service] CreateWebRTCSession request: node_id=%s, service_id=%s",
		req.Msg.NodeId, req.Msg.ServiceId)

	// Validate request
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}
	if req.Msg.ServiceUrl == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_url is required"))
	}
	if req.Msg.SdpOffer == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("sdp_offer is required"))
	}

	// Get node connection
	nodeConn, exists := s.server.GetNodeConnection(req.Msg.NodeId)
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s not found or not connected", req.Msg.NodeId))
	}

	// Create WebRTC session
	session, sdpAnswer, err := NewSession(
		ctx,
		nodeConn,
		req.Msg.ServiceId,
		req.Msg.ServiceUrl,
		req.Msg.SdpOffer,
		s.sessionMgr,
	)
	if err != nil {
		log.Printf("[WebRTC Service] Failed to create session: %v", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create session: %w", err))
	}

	log.Printf("[WebRTC Service] Session %s created successfully", session.SessionID)

	resp := &webrtcv1.CreateWebRTCSessionResponse{
		SdpAnswer: sdpAnswer,
		SessionId: session.SessionID,
	}

	return connect.NewResponse(resp), nil
}

// GetSessionManager returns the session manager for external access if needed
func (s *Service) GetSessionManager() *SessionManager {
	return s.sessionMgr
}

// Shutdown closes all active sessions
func (s *Service) Shutdown() {
	log.Printf("[WebRTC Service] Shutting down, closing all sessions")
	s.sessionMgr.Close()
}
