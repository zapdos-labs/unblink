package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"connectrpc.com/connect"
	servicev1 "unb/server/gen/service/v1"
	"unb/server/gen/service/v1/servicev1connect"
)

// StorageDatabase defines the interface for storage database operations
type StorageDatabase interface {
	ListFramesForService(serviceID string, limit, offset int64) ([]*servicev1.Frame, int64, error)
	GetFrameInfo(frameID string) (*servicev1.Frame, error)
	ListServicesWithFrames(nodeID string) ([]*servicev1.ServiceFrames, error)
	DeleteOldFrames(serviceID string, olderThanSeconds int64) (int64, error)
}

// StorageConfig holds configuration for storage
type StorageConfig struct {
	FramesBaseDir string // Base directory where frames are stored
}

// StorageService handles storage operations
type StorageService struct {
	db     StorageDatabase
	config *StorageConfig
}

func NewStorageService(db StorageDatabase, config *StorageConfig) *StorageService {
	return &StorageService{
		db:     db,
		config: config,
	}
}

// ListFrames lists frames for a service
func (s *StorageService) ListFrames(ctx context.Context, req *connect.Request[servicev1.ListFramesRequest]) (*connect.Response[servicev1.ListFramesResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 100 // Default limit
	}
	if limit > 1000 {
		limit = 1000 // Max limit
	}

	frames, total, err := s.db.ListFramesForService(req.Msg.ServiceId, limit, req.Msg.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list frames: %w", err))
	}

	log.Printf("[Storage] Listed %d frames for service %s (total: %d)", len(frames), req.Msg.ServiceId, total)

	return connect.NewResponse(&servicev1.ListFramesResponse{
		Frames: frames,
		Total:  total,
	}), nil
}

// GetFrame gets metadata for a specific frame
func (s *StorageService) GetFrame(ctx context.Context, req *connect.Request[servicev1.GetFrameRequest]) (*connect.Response[servicev1.GetFrameResponse], error) {
	if req.Msg.FrameId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("frame_id is required"))
	}

	frame, err := s.db.GetFrameInfo(req.Msg.FrameId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("frame not found: %w", err))
	}

	log.Printf("[Storage] Got frame %s for service %s", frame.Id, frame.ServiceId)

	return connect.NewResponse(&servicev1.GetFrameResponse{
		Frame: frame,
	}), nil
}

// ListServicesWithFrames lists services with their frames for a node
func (s *StorageService) ListServicesWithFrames(ctx context.Context, req *connect.Request[servicev1.ListServicesWithFramesRequest]) (*connect.Response[servicev1.ListServicesWithFramesResponse], error) {
	if req.Msg.NodeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node_id is required"))
	}

	services, err := s.db.ListServicesWithFrames(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list services with frames: %w", err))
	}

	log.Printf("[Storage] Listed %d services with frames for node %s", len(services), req.Msg.NodeId)

	return connect.NewResponse(&servicev1.ListServicesWithFramesResponse{
		Services: services,
	}), nil
}

// DeleteOldFrames deletes frames older than the specified duration
func (s *StorageService) DeleteOldFrames(ctx context.Context, req *connect.Request[servicev1.DeleteOldFramesRequest]) (*connect.Response[servicev1.DeleteOldFramesResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	if req.Msg.OlderThanSeconds <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("older_than_seconds must be positive"))
	}

	deletedCount, err := s.db.DeleteOldFrames(req.Msg.ServiceId, req.Msg.OlderThanSeconds)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete old frames: %w", err))
	}

	log.Printf("[Storage] Deleted %d frames for service %s (older than %d seconds)", deletedCount, req.Msg.ServiceId, req.Msg.OlderThanSeconds)

	return connect.NewResponse(&servicev1.DeleteOldFramesResponse{
		DeletedCount: deletedCount,
	}), nil
}

// Ensure StorageService implements interface
var _ servicev1connect.StorageServiceHandler = (*StorageService)(nil)

// RegisterHTTPHandlers registers HTTP handlers for serving JPEG files
func (s *StorageService) RegisterHTTPHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/frames/", s.serveFrame)
	log.Printf("[Storage] Registered HTTP handler for /frames/")
}

// serveFrame serves JPEG files via HTTP
func (s *StorageService) serveFrame(w http.ResponseWriter, r *http.Request) {
	// Extract path components: /frames/{serviceID}/{frameID}.jpg
	path := r.URL.Path
	if len(path) < 9 || path[:9] != "/frames/" {
		http.NotFound(w, r)
		return
	}

	rest := path[9:] // Remove "/frames/"
	// Find the second slash
	for i, c := range rest {
		if c == '/' {
			serviceID := rest[:i]
			frameName := rest[i+1:]
			if frameName == "" {
				http.NotFound(w, r)
				return
			}

			// Build file path
			framePath := s.config.FramesBaseDir + "/" + serviceID + "/" + frameName

			// Check if file exists
			if _, err := os.Stat(framePath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}

			// Serve file
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
			http.ServeFile(w, r, framePath)
			return
		}
	}

	http.NotFound(w, r)
}
