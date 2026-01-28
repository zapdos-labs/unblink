package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	servicev1 "unblink/server/gen/service/v1"
	"unblink/server/gen/service/v1/servicev1connect"
	"unblink/server/internal/ctxutil"
)

// generateID creates a unique ID using crypto/rand
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Database defines the interface for service database operations
type Database interface {
	CreateService(id, name, url, nodeID string) error
	GetService(id string) (*servicev1.Service, error)
	ListServicesByNodeId(nodeID string) ([]*servicev1.Service, error)
	UpdateService(id, name, url string) error
	DeleteService(id string) error
	CheckNodeAccess(nodeID, userID string) (bool, error)
}

type Service struct {
	db       Database
	registry *ServiceRegistry
}

func NewService(db Database, registry *ServiceRegistry) *Service {
	return &Service{
		db:       db,
		registry: registry,
	}
}

// CreateService creates a new service
func (s *Service) CreateService(ctx context.Context, req *connect.Request[servicev1.CreateServiceRequest]) (*connect.Response[servicev1.CreateServiceResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, req.Msg.NodeId); err != nil {
		return nil, err
	}

	id := generateID()
	now := time.Now()

	name := req.Msg.Name
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	url := req.Msg.Url
	if url == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("url is required"))
	}

	nodeID := req.Msg.NodeId

	err := s.db.CreateService(id, name, url, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create service: %w", err))
	}

	// Notify registry
	if s.registry != nil {
		s.registry.AddService(&servicev1.Service{
			Id:     id,
			Name:   name,
			Url:    url,
			NodeId: nodeID,
		})
	}

	log.Printf("[Service] Created service: id=%s, name=%s, url=%s, node_id=%s", id, name, url, nodeID)

	return connect.NewResponse(&servicev1.CreateServiceResponse{
		Service: &servicev1.Service{
			Id:        id,
			Name:      name,
			Url:       url,
			NodeId:    nodeID,
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}), nil
}

// ListServicesByNodeId retrieves all services for a node
func (s *Service) ListServicesByNodeId(ctx context.Context, req *connect.Request[servicev1.ListServicesByNodeIdRequest]) (*connect.Response[servicev1.ListServicesByNodeIdResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, req.Msg.NodeId); err != nil {
		return nil, err
	}

	services, err := s.db.ListServicesByNodeId(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list services: %w", err))
	}

	return connect.NewResponse(&servicev1.ListServicesByNodeIdResponse{
		Services: services,
	}), nil
}

// UpdateService updates an existing service
func (s *Service) UpdateService(ctx context.Context, req *connect.Request[servicev1.UpdateServiceRequest]) (*connect.Response[servicev1.UpdateServiceResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	name := req.Msg.Name
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
	}

	url := req.Msg.Url
	if url == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("url is required"))
	}

	// Get the existing service to get node_id
	existingService, err := s.db.GetService(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, existingService.NodeId); err != nil {
		return nil, err
	}

	// Update the service
	err = s.db.UpdateService(req.Msg.Id, name, url)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update service: %w", err))
	}

	// Notify registry
	if s.registry != nil {
		s.registry.UpdateService(&servicev1.Service{
			Id:     req.Msg.Id,
			Name:   name,
			Url:    url,
			NodeId: existingService.NodeId,
		})
	}

	log.Printf("[Service] Updated service: id=%s, name=%s, url=%s", req.Msg.Id, name, url)

	return connect.NewResponse(&servicev1.UpdateServiceResponse{
		Service: &servicev1.Service{
			Id:        req.Msg.Id,
			Name:      name,
			Url:       url,
			NodeId:    existingService.NodeId,
			CreatedAt: existingService.CreatedAt,
			UpdatedAt: timestamppb.New(time.Now()),
		},
	}), nil
}

// DeleteService deletes a service by ID
func (s *Service) DeleteService(ctx context.Context, req *connect.Request[servicev1.DeleteServiceRequest]) (*connect.Response[servicev1.DeleteServiceResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	// Get the service to check node ownership
	service, err := s.db.GetService(req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Verify node access first
	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, service.NodeId); err != nil {
		return nil, err
	}

	err = s.db.DeleteService(req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete service: %w", err))
	}

	// Notify registry
	if s.registry != nil {
		s.registry.RemoveService(req.Msg.ServiceId)
	}

	log.Printf("[Service] Deleted service: id=%s", req.Msg.ServiceId)

	return connect.NewResponse(&servicev1.DeleteServiceResponse{
		Success: true,
	}), nil
}

// Ensure Service implements interface
var _ servicev1connect.ServiceServiceHandler = (*Service)(nil)
