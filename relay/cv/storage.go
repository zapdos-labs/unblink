package cv

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FrameMetadata contains metadata about a stored frame
type FrameMetadata struct {
	UUID      string
	ServiceID string
	FilePath  string
	Timestamp time.Time
	FileSize  int64
	CreatedAt time.Time
}

// StorageManager manages frame storage and file serving
type StorageManager struct {
	baseDir        string
	frames         map[string]*FrameMetadata // UUID → metadata
	workerRegistry *CVWorkerRegistry         // For key validation
	mu             sync.RWMutex
}

// NewStorageManager creates a new storage manager
func NewStorageManager(baseDir string, workerRegistry *CVWorkerRegistry) *StorageManager {
	// Create frames directory
	framesDir := filepath.Join(baseDir, "frames")
	if err := os.MkdirAll(framesDir, 0755); err != nil {
		log.Printf("[StorageManager] Failed to create frames directory: %v", err)
	}

	return &StorageManager{
		baseDir:        baseDir,
		frames:         make(map[string]*FrameMetadata),
		workerRegistry: workerRegistry,
	}
}

// RegisterFrame registers a new frame
func (s *StorageManager) RegisterFrame(metadata *FrameMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.frames[metadata.UUID] = metadata
	log.Printf("[StorageManager] Registered frame %s for service %s", metadata.UUID, metadata.ServiceID)
	return nil
}

// GetFrame retrieves frame metadata by UUID
func (s *StorageManager) GetFrame(uuid string) (*FrameMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	frame, exists := s.frames[uuid]
	if !exists {
		return nil, fmt.Errorf("frame not found: %s", uuid)
	}

	return frame, nil
}

// GetFramesDir returns the frames directory path
func (s *StorageManager) GetFramesDir() string {
	return filepath.Join(s.baseDir, "frames")
}

// ValidateWorkerKey validates that a worker key is valid
func (s *StorageManager) ValidateWorkerKey(workerKey string) (string, error) {
	// Get workerID from key
	workerID, exists := s.workerRegistry.GetWorkerIDByKey(workerKey)
	if !exists {
		return "", fmt.Errorf("invalid worker key")
	}
	return workerID, nil
}

// HandleFrameDownload handles HTTP requests for frame downloads
// URL format: /frames/{frameUUID}
func (s *StorageManager) HandleFrameDownload(w http.ResponseWriter, r *http.Request) {
	// Parse path: /frames/{frameUUID}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "frames" {
		http.Error(w, "Invalid path format. Expected: /frames/{frameUUID}", http.StatusBadRequest)
		return
	}

	frameUUID := parts[1]

	// Get worker key from header
	workerKey := r.Header.Get("X-Worker-Key")
	if workerKey == "" {
		http.Error(w, "Missing X-Worker-Key header", http.StatusUnauthorized)
		return
	}

	// Validate worker key
	workerID, err := s.ValidateWorkerKey(workerKey)
	if err != nil {
		log.Printf("[StorageManager] Invalid worker key for frame %s", frameUUID)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get frame metadata
	frame, err := s.GetFrame(frameUUID)
	if err != nil {
		http.Error(w, "Frame not found", http.StatusNotFound)
		return
	}

	// Check if file exists
	if _, err := os.Stat(frame.FilePath); os.IsNotExist(err) {
		http.Error(w, "File not found on disk", http.StatusNotFound)
		return
	}

	log.Printf("[StorageManager] Worker %s downloading frame %s", workerID, frameUUID)

	// Serve file
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.jpg", frameUUID))
	http.ServeFile(w, r, frame.FilePath)
}
