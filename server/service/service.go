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

	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"
	"github.com/zapdos-labs/unblink/server/gen/service/v1/servicev1connect"
	"github.com/zapdos-labs/unblink/server/internal/ctxutil"
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
	CreateSOPProcedure(id, nodeID, title, content string) error
	GetSOPProcedure(id string) (*servicev1.SOPProcedure, error)
	ListSOPProceduresByNodeID(nodeID string) ([]*servicev1.SOPProcedure, error)
	UpdateSOPProcedure(id, title, content string) error
	DeleteSOPProcedure(id string) error
	CheckNodeAccess(nodeID, userID string) (bool, error)
	IsGuest(userID string) (bool, error)
	AssociateUserNode(userID, nodeID string) error
	ListUserNodes(userID string) ([]string, error)
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
		Services:   services,
		NodeOnline: s.registry.IsNodeOnline(req.Msg.NodeId),
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

func (s *Service) CreateSOPProcedure(ctx context.Context, req *connect.Request[servicev1.CreateSOPProcedureRequest]) (*connect.Response[servicev1.CreateSOPProcedureResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}
	title := req.Msg.Title
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title is required"))
	}
	content := req.Msg.Content
	if content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}

	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, req.Msg.NodeId); err != nil {
		return nil, err
	}

	id := generateID()
	if err := s.db.CreateSOPProcedure(id, req.Msg.NodeId, title, content); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create SOP procedure: %w", err))
	}

	created, err := s.db.GetSOPProcedure(id)
	if err != nil || created == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load SOP procedure after create: %w", err))
	}

	log.Printf("[Service] Created SOP procedure: id=%s node_id=%s", id, req.Msg.NodeId)
	return connect.NewResponse(&servicev1.CreateSOPProcedureResponse{
		Procedure: created,
	}), nil
}

func (s *Service) ListSOPProceduresByNodeId(ctx context.Context, req *connect.Request[servicev1.ListSOPProceduresByNodeIdRequest]) (*connect.Response[servicev1.ListSOPProceduresByNodeIdResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, req.Msg.NodeId); err != nil {
		return nil, err
	}

	procedures, err := s.db.ListSOPProceduresByNodeID(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list SOP procedures: %w", err))
	}

	return connect.NewResponse(&servicev1.ListSOPProceduresByNodeIdResponse{
		Procedures: procedures,
	}), nil
}

func (s *Service) UpdateSOPProcedure(ctx context.Context, req *connect.Request[servicev1.UpdateSOPProcedureRequest]) (*connect.Response[servicev1.UpdateSOPProcedureResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}
	title := req.Msg.Title
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title is required"))
	}
	content := req.Msg.Content
	if content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}

	existing, err := s.db.GetSOPProcedure(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load SOP procedure: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("SOP procedure not found"))
	}

	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, existing.NodeId); err != nil {
		return nil, err
	}

	if err := s.db.UpdateSOPProcedure(req.Msg.Id, title, content); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update SOP procedure: %w", err))
	}

	updated, err := s.db.GetSOPProcedure(req.Msg.Id)
	if err != nil || updated == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load SOP procedure after update: %w", err))
	}

	log.Printf("[Service] Updated SOP procedure: id=%s node_id=%s", req.Msg.Id, updated.NodeId)
	return connect.NewResponse(&servicev1.UpdateSOPProcedureResponse{
		Procedure: updated,
	}), nil
}

func (s *Service) DeleteSOPProcedure(ctx context.Context, req *connect.Request[servicev1.DeleteSOPProcedureRequest]) (*connect.Response[servicev1.DeleteSOPProcedureResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("id is required"))
	}

	existing, err := s.db.GetSOPProcedure(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load SOP procedure: %w", err))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("SOP procedure not found"))
	}

	if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, existing.NodeId); err != nil {
		return nil, err
	}

	if err := s.db.DeleteSOPProcedure(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete SOP procedure: %w", err))
	}

	log.Printf("[Service] Deleted SOP procedure: id=%s node_id=%s", req.Msg.Id, existing.NodeId)
	return connect.NewResponse(&servicev1.DeleteSOPProcedureResponse{Success: true}), nil
}

// AssociateUserNode associates a node with the authenticated user
func (s *Service) AssociateUserNode(ctx context.Context, req *connect.Request[servicev1.AssociateUserNodeRequest]) (*connect.Response[servicev1.AssociateUserNodeResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	// Get authenticated user ID from context
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Only allow non-guest users to associate nodes
	isGuest, err := s.db.IsGuest(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check guest status: %w", err))
	}
	if isGuest {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("guest users cannot associate nodes"))
	}

	// Associate the node with the user
	err = s.db.AssociateUserNode(userID, req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to associate user with node: %w", err))
	}

	log.Printf("[Service] Associated user_id=%s with node_id=%s", userID, req.Msg.NodeId)

	return connect.NewResponse(&servicev1.AssociateUserNodeResponse{
		Success: true,
	}), nil
}

// ListUserNodes lists all nodes associated with the authenticated user
func (s *Service) ListUserNodes(ctx context.Context, req *connect.Request[servicev1.ListUserNodesRequest]) (*connect.Response[servicev1.ListUserNodesResponse], error) {
	// Get authenticated user ID from context
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	nodeIDs, err := s.db.ListUserNodes(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list user nodes: %w", err))
	}

	return connect.NewResponse(&servicev1.ListUserNodesResponse{
		NodeIds: nodeIDs,
	}), nil
}

// Ensure Service implements interface
var _ servicev1connect.ServiceServiceHandler = (*Service)(nil)
