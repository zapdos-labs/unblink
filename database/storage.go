package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StorageType represents the type of stored content
type StorageType string

const (
	StorageTypeFrame  StorageType = "frame"  // JPEG video frame
	StorageTypeClip   StorageType = "clip"   // Video clip
	StorageTypeSnapshot StorageType = "snapshot" // Snapshot image
)

// FrameMetadata holds additional metadata for frames
type FrameMetadata struct {
	Sequence int64  `json:"sequence"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// StorageEntry represents a row in the storage table
type StorageEntry struct {
	ID          string
	ServiceID   string
	Type        StorageType
	StoragePath string
	Timestamp   time.Time
	FileSize    int64
	ContentType string
	CreatedAt   time.Time
	Metadata    string // JSON-encoded metadata
}

// SaveFrame saves frame metadata to the database
func (c *Client) SaveFrame(serviceID, storagePath string, timestamp time.Time, fileSize int64, metadata *FrameMetadata) error {
	id := uuid.New().String()

	var metadataJSON sql.NullString
	if metadata != nil {
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal frame metadata: %w", err)
		}
		metadataJSON = sql.NullString{String: string(metadataBytes), Valid: true}
	}

	insertSQL := `
		INSERT INTO storage (id, service_id, type, storage_path, timestamp, file_size, content_type, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := c.db.Exec(insertSQL,
		id, serviceID, StorageTypeFrame, storagePath, timestamp, fileSize, "image/jpeg", metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save frame metadata: %w", err)
	}

	return nil
}

// ListStorageByService lists storage entries for a service from the database
func (c *Client) ListStorageByService(serviceID string, storageType StorageType, limit, offset int64) ([]*StorageEntry, int64, error) {
	// Get total count
	var total int64
	countSQL := `SELECT COUNT(*) FROM storage WHERE service_id = $1`
	if storageType != "" {
		countSQL += ` AND type = $2`
	}

	err := c.db.QueryRow(countSQL, serviceID, storageType).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count storage entries: %w", err)
	}

	// Query entries
	querySQL := `
		SELECT id, service_id, type, storage_path, timestamp, file_size, content_type, created_at, metadata
		FROM storage
		WHERE service_id = $1
	`
	args := []interface{}{serviceID}

	argOffset := 2
	if storageType != "" {
		querySQL += fmt.Sprintf(` AND type = $%d`, argOffset)
		args = append(args, storageType)
		argOffset++
	}

	querySQL += fmt.Sprintf(` ORDER BY timestamp DESC LIMIT $%d OFFSET $%d`, argOffset, argOffset+1)
	args = append(args, limit, offset)

	rows, err := c.db.Query(querySQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list storage entries: %w", err)
	}
	defer rows.Close()

	var entries []*StorageEntry
	for rows.Next() {
		var entry StorageEntry
		var metadata sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.ServiceID,
			&entry.Type,
			&entry.StoragePath,
			&entry.Timestamp,
			&entry.FileSize,
			&entry.ContentType,
			&entry.CreatedAt,
			&metadata,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan storage entry: %w", err)
		}

		if metadata.Valid {
			entry.Metadata = metadata.String
		}

		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating storage entries: %w", err)
	}

	return entries, total, nil
}

// DeleteOldStorage deletes storage entries older than the specified duration
func (c *Client) DeleteOldStorage(serviceID string, storageType StorageType, olderThanSeconds int64) (int64, error) {
	deleteSQL := `
		DELETE FROM storage
		WHERE service_id = $1
		AND type = $2
		AND created_at < $3
	`

	cutoffTime := time.Now().Add(-time.Duration(olderThanSeconds) * time.Second)

	result, err := c.db.Exec(deleteSQL, serviceID, storageType, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old storage entries: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// DeleteStorageByPath deletes a storage entry by its file path
func (c *Client) DeleteStorageByPath(serviceID, storagePath string) error {
	deleteSQL := `DELETE FROM storage WHERE service_id = $1 AND storage_path = $2`

	_, err := c.db.Exec(deleteSQL, serviceID, storagePath)
	if err != nil {
		return fmt.Errorf("failed to delete storage entry: %w", err)
	}

	return nil
}
