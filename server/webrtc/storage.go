package webrtc

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Storage handles saving frames to disk
type Storage struct {
	baseDir string // Base directory for frame storage (empty = no storage)
	onSaved func(serviceID, frameID string, framePath string, timestamp time.Time, fileSize int64, sequence int64) // Callback after frame is saved
}

// NewStorage creates a new storage handler
func NewStorage(baseDir string) *Storage {
	return &Storage{
		baseDir: baseDir,
	}
}

// SetOnSaved sets the callback to be called after a frame is saved
func (fs *Storage) SetOnSaved(onSaved func(serviceID, frameID string, framePath string, timestamp time.Time, fileSize int64, sequence int64)) {
	fs.onSaved = onSaved
}

// Save saves a frame to disk if baseDir is configured
func (fs *Storage) Save(serviceID string, frame *Frame) {
	if fs.baseDir == "" {
		log.Printf("[Storage] Skipping frame save for service %s - no baseDir configured", serviceID)
		return
	}

	serviceFramesDir := filepath.Join(fs.baseDir, serviceID)
	if err := os.MkdirAll(serviceFramesDir, 0755); err != nil {
		log.Printf("[Storage] Failed to create frames directory %s: %v", serviceFramesDir, err)
		return
	}

	frameID := uuid.New().String()
	framePath := filepath.Join(serviceFramesDir, frameID+".jpg")
	if err := os.WriteFile(framePath, frame.Data, 0644); err != nil {
		log.Printf("[Storage] Failed to save frame %s: %v", frameID, err)
	} else {
		log.Printf("[Storage] Saved frame seq=%d service=%s size=%d path=%s", frame.Sequence, serviceID, len(frame.Data), framePath)

		// Call callback if registered (e.g., to save metadata to database)
		if fs.onSaved != nil {
			// Note: We're passing frame.Timestamp which is when the frame was captured
			// The database should use created_at as the DB insertion time
			fs.onSaved(serviceID, frameID, framePath, frame.Timestamp, int64(len(frame.Data)), frame.Sequence)
		}
	}
}
