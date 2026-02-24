package webrtc

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"

	"unblink/database"
	"unblink/server"
	"unblink/server/internal/ctxutil"
	webrtcv1 "unblink/server/gen/webrtc/v1"
)

// Service implements the WebRTC gRPC/Connect service
type Service struct {
	server     *server.Server
	sessionMgr *SessionManager
	db         *database.Client
}

// NewService creates a new WebRTC service
func NewService(srv *server.Server, db *database.Client) *Service {
	return &Service{
		server:     srv,
		sessionMgr: NewSessionManager(),
		db:         db,
	}
}

// verifyServiceAccess checks if the user can access the service via node ownership
func (s *Service) verifyServiceAccess(ctx context.Context, nodeID string) error {
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Check node access
	hasAccess, err := s.db.CheckNodeAccess(nodeID, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to verify node access: %w", err))
	}

	if !hasAccess {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you don't have access to this node"))
	}

	return nil
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

	// Verify node access first
	if err := s.verifyServiceAccess(ctx, req.Msg.NodeId); err != nil {
		return nil, err
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
