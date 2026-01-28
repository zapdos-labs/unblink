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
	ListEventsByServiceId(serviceID string) ([]*servicev1.Event, error)
	CheckNodeAccess(nodeID, userID string) (bool, error)
}

type EventService struct {
	db EventDatabase
}

func NewEventService(db EventDatabase) *EventService {
	return &EventService{
		db: db,
	}
}

// ListEventsByServiceId retrieves all events for a service
func (s *EventService) ListEventsByServiceId(ctx context.Context, req *connect.Request[servicev1.ListEventsByServiceIdRequest]) (*connect.Response[servicev1.ListEventsByServiceIdResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	// Get the service to check node ownership
	svc, err := s.db.GetService(req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, svc.NodeId); err != nil {
		return nil, err
	}

	events, err := s.db.ListEventsByServiceId(req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list events: %w", err))
	}

	return connect.NewResponse(&servicev1.ListEventsByServiceIdResponse{
		Events: events,
	}), nil
}

// Ensure EventService implements interface
var _ servicev1connect.EventServiceHandler = (*EventService)(nil)
