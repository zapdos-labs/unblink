package service

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	servicev1 "unblink/server/gen/service/v1"
	"unblink/server/gen/service/v1/servicev1connect"
	"unblink/server/internal/ctxutil"
)

// EventDatabase defines the interface for event database operations
type EventDatabase interface {
	GetService(id string) (*servicev1.Service, error)
	ListEventsByNodeId(nodeID string, pageSize, pageOffset int32) ([]*servicev1.Event, int32, error)
	CheckNodeAccess(nodeID, userID string) (bool, error)
	CountEventsForUser(userID string) (int64, error)
}

type EventService struct {
	db EventDatabase
}

func NewEventService(db EventDatabase) *EventService {
	return &EventService{
		db: db,
	}
}

// ListEventsByNodeId retrieves events for all services in a node with pagination
func (s *EventService) ListEventsByNodeId(ctx context.Context, req *connect.Request[servicev1.ListEventsByNodeIdRequest]) (*connect.Response[servicev1.ListEventsByNodeIdResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, req.Msg.NodeId); err != nil {
		return nil, err
	}

	pageSize := req.Msg.PageSize
	pageOffset := req.Msg.PageOffset

	events, totalCount, err := s.db.ListEventsByNodeId(req.Msg.NodeId, pageSize, pageOffset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list events: %w", err))
	}

	return connect.NewResponse(&servicev1.ListEventsByNodeIdResponse{
		Events:     events,
		TotalCount: totalCount,
	}), nil
}

// CountEventsForUser counts all events accessible to the authenticated user
func (s *EventService) CountEventsForUser(ctx context.Context, req *connect.Request[servicev1.CountEventsForUserRequest]) (*connect.Response[servicev1.CountEventsForUserResponse], error) {
	// Get authenticated user ID from context
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	count, err := s.db.CountEventsForUser(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count events: %w", err))
	}

	return connect.NewResponse(&servicev1.CountEventsForUserResponse{
		Count: count,
	}), nil
}

// Ensure EventService implements interface
var _ servicev1connect.EventServiceHandler = (*EventService)(nil)
