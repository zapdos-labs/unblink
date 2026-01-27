package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	servicev1 "unb/server/gen/service/v1"
)

// ListFramesForService lists all stored frames for a service, sorted by timestamp (newest first)
func (c *Client) ListFramesForService(serviceID string, limit, offset int64) ([]*servicev1.Frame, int64, error) {
	entries, total, err := c.ListStorageByService(serviceID, StorageTypeFrame, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*servicev1.Frame, 0, len(entries))

	for _, entry := range entries {
		// Parse metadata if available
		var sequence int64
		if entry.Metadata != "" {
			var metadata FrameMetadata
			if err := json.Unmarshal([]byte(entry.Metadata), &metadata); err == nil {
				sequence = metadata.Sequence
			}
		}

		// Create relative path for HTTP access from storage_path
		// storage_path is absolute, convert to relative
		baseDir := c.getFramesBaseDir()
		var relPath string
		if strings.HasPrefix(entry.StoragePath, baseDir) {
			relPath = entry.StoragePath[len(baseDir):]
		} else {
			relPath = entry.StoragePath
		}

		result = append(result, &servicev1.Frame{
			Id:        entry.ID,
			ServiceId: entry.ServiceID,
			Sequence:  sequence,
			Size:      entry.FileSize,
			Timestamp: timestamppb.New(entry.Timestamp),
			Path:      relPath,
		})
	}

	return result, total, nil
}

// GetFrameInfo gets metadata for a specific frame by ID
func (c *Client) GetFrameInfo(frameID string) (*servicev1.Frame, error) {
	querySQL := `
		SELECT id, service_id, storage_path, timestamp, file_size, metadata
		FROM storage
		WHERE id = $1 AND type = $2
	`

	var id, storagePathStr, serviceIDStr string
	var metadata sql.NullString
	var timestamp time.Time
	var fileSize int64

	err := c.db.QueryRow(querySQL, frameID, StorageTypeFrame).Scan(
		&id,
		&serviceIDStr,
		&storagePathStr,
		&timestamp,
		&fileSize,
		&metadata,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("frame not found")
		}
		return nil, fmt.Errorf("failed to get frame info: %w", err)
	}

	// Verify file exists on disk
	if _, err := os.Stat(storagePathStr); os.IsNotExist(err) {
		return nil, fmt.Errorf("frame file not found on disk: %s", storagePathStr)
	}

	// Parse metadata for sequence number
	var sequence int64
	if metadata.Valid && metadata.String != "" {
		var frameMetadata FrameMetadata
		if err := json.Unmarshal([]byte(metadata.String), &frameMetadata); err == nil {
			sequence = frameMetadata.Sequence
		}
	}

	// Create relative path for HTTP access
	baseDir := c.getFramesBaseDir()
	var relPath string
	if strings.HasPrefix(storagePathStr, baseDir) {
		relPath = storagePathStr[len(baseDir):]
	} else {
		relPath = storagePathStr
	}

	return &servicev1.Frame{
		Id:        id,
		ServiceId: serviceIDStr,
		Sequence:  sequence,
		Size:      fileSize,
		Timestamp: timestamppb.New(timestamp),
		Path:      relPath,
	}, nil
}

// ListServicesWithFrames lists all services for a node along with their latest frames
func (c *Client) ListServicesWithFrames(nodeID string) ([]*servicev1.ServiceFrames, error) {
	services, err := c.ListServicesByNodeId(nodeID, "")
	if err != nil {
		return nil, err
	}

	result := make([]*servicev1.ServiceFrames, 0, len(services))

	for _, svc := range services {
		frames, total, err := c.ListFramesForService(svc.Id, 10, 0) // Get latest 10 frames
		if err != nil {
			// Skip services with frame errors
			continue
		}

		result = append(result, &servicev1.ServiceFrames{
			Service:      svc,
			Frames:       frames,
			TotalFrames:  total,
		})
	}

	return result, nil
}

// DeleteOldFrames deletes frames (both DB entries and files) older than the specified duration
func (c *Client) DeleteOldFrames(serviceID string, olderThanSeconds int64) (int64, error) {
	baseDir := c.getFramesBaseDir()
	if baseDir == "" {
		return 0, fmt.Errorf("frames base dir not configured")
	}

	// First, get the storage entries to delete
	querySQL := `
		SELECT id, storage_path
		FROM storage
		WHERE service_id = $1
		AND type = $2
		AND created_at < $3
		ORDER BY created_at
	`

	cutoffTime := time.Now().Add(-time.Duration(olderThanSeconds) * time.Second)

	rows, err := c.db.Query(querySQL, serviceID, StorageTypeFrame, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to query old frames: %w", err)
	}
	defer rows.Close()

	var deletedCount int64

	for rows.Next() {
		var id, storagePath string
		if err := rows.Scan(&id, &storagePath); err != nil {
			continue
		}

		// Delete file from disk
		if err := os.Remove(storagePath); err != nil {
			// Log error but continue
			fmt.Printf("[DeleteOldFrames] Failed to delete file %s: %v\n", storagePath, err)
		}

		// Delete from database by path (since we have the path, use that)
		if err := c.DeleteStorageByPath(serviceID, storagePath); err != nil {
			fmt.Printf("[DeleteOldFrames] Failed to delete DB entry for %s: %v\n", id, err)
			continue
		}

		deletedCount++
	}

	if err := rows.Err(); err != nil {
		return deletedCount, fmt.Errorf("error iterating old frames: %w", err)
	}

	return deletedCount, nil
}

// getFramesBaseDir returns the base directory for frame storage
func (c *Client) getFramesBaseDir() string {
	// This should be configured from the config
	// For now, use a default path
	return "/home/tri/unb-data/storage/frames"
}
