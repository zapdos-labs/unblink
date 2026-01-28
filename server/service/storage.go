package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"unblink/database"
	servicev1 "unblink/server/gen/service/v1"
	"unblink/server/gen/service/v1/servicev1connect"
)

// StorageDatabase defines the interface for storage database operations
type StorageDatabase interface {
	ListStorageItemsForService(serviceID string, storageType string, limit, offset int64) ([]*database.StorageEntry, int64, error)
	GetStorageItemInfo(itemID string) (*database.StorageEntry, error)
	DeleteOldStorageItems(serviceID string, storageType string, olderThanSeconds int64) (int64, error)
}

// StorageConfig holds configuration for storage
type StorageConfig struct {
	StorageBaseDir string // Base directory where storage items are stored
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

// ListStorageItems lists storage items for a service
func (s *StorageService) ListStorageItems(ctx context.Context, req *connect.Request[servicev1.ListStorageItemsRequest]) (*connect.Response[servicev1.ListStorageItemsResponse], error) {
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

	// Use type from request (empty string = all types)
	storageType := req.Msg.Type

	entries, total, err := s.db.ListStorageItemsForService(req.Msg.ServiceId, storageType, limit, req.Msg.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list storage items: %w", err))
	}

	// Convert to proto format
	items := make([]*servicev1.StorageItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, &servicev1.StorageItem{
			Id:        entry.ID,
			ServiceId: entry.ServiceID,
			Type:      string(entry.Type),
			Size:      entry.FileSize,
			Timestamp: timestamppb.New(entry.Timestamp),
		})
	}

	log.Printf("[Storage] Listed %d storage items for service %s (total: %d)", len(items), req.Msg.ServiceId, total)

	return connect.NewResponse(&servicev1.ListStorageItemsResponse{
		Items: items,
		Total: total,
	}), nil
}

// GetStorageItem gets metadata for a specific storage item
func (s *StorageService) GetStorageItem(ctx context.Context, req *connect.Request[servicev1.GetStorageItemRequest]) (*connect.Response[servicev1.GetStorageItemResponse], error) {
	if req.Msg.ItemId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("item_id is required"))
	}

	entry, err := s.db.GetStorageItemInfo(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("storage item not found: %w", err))
	}

	// Verify file exists on disk
	if _, err := os.Stat(entry.StoragePath); os.IsNotExist(err) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("storage item file not found on disk"))
	}

	item := &servicev1.StorageItem{
		Id:        entry.ID,
		ServiceId: entry.ServiceID,
		Type:      string(entry.Type),
		Size:      entry.FileSize,
		Timestamp: timestamppb.New(entry.Timestamp),
	}

	log.Printf("[Storage] Got storage item %s for service %s", item.Id, item.ServiceId)

	return connect.NewResponse(&servicev1.GetStorageItemResponse{
		Item: item,
	}), nil
}

// DeleteOldStorageItems deletes storage items older than the specified duration
func (s *StorageService) DeleteOldStorageItems(ctx context.Context, req *connect.Request[servicev1.DeleteOldStorageItemsRequest]) (*connect.Response[servicev1.DeleteOldStorageItemsResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	if req.Msg.OlderThanSeconds <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("older_than_seconds must be positive"))
	}

	// Use type from request (empty string = all types)
	storageType := req.Msg.Type

	deletedCount, err := s.db.DeleteOldStorageItems(req.Msg.ServiceId, storageType, req.Msg.OlderThanSeconds)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete old storage items: %w", err))
	}

	log.Printf("[Storage] Deleted %d storage items for service %s (older than %d seconds)", deletedCount, req.Msg.ServiceId, req.Msg.OlderThanSeconds)

	return connect.NewResponse(&servicev1.DeleteOldStorageItemsResponse{
		DeletedCount: deletedCount,
	}), nil
}

// Ensure StorageService implements interface
var _ servicev1connect.StorageServiceHandler = (*StorageService)(nil)

// RegisterHTTPHandlers registers HTTP handlers for serving JPEG files
func (s *StorageService) RegisterHTTPHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/storage/", s.serveStorage)
	log.Printf("[Storage] Registered HTTP handler for /storage/")
}

// serveStorage serves JPEG files via HTTP
// URL format: /storage/{itemID}
func (s *StorageService) serveStorage(w http.ResponseWriter, r *http.Request) {
	// Extract itemID from path: /storage/{itemID}
	path := r.URL.Path
	if len(path) < 10 || path[:10] != "/storage/" {
		http.NotFound(w, r)
		return
	}

	itemID := path[10:] // Remove "/storage/"
	if itemID == "" {
		http.NotFound(w, r)
		return
	}

	// Look up item in database to get file path
	entry, err := s.db.GetStorageItemInfo(itemID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check if file exists on disk
	if _, err := os.Stat(entry.StoragePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Serve file
	w.Header().Set("Content-Type", entry.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	http.ServeFile(w, r, entry.StoragePath)
}
