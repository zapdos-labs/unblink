package database

import (
	"database/sql"
	"fmt"
	"time"

	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"
)

const (
	createSOPProcedureTablesSQL = `
		CREATE TABLE IF NOT EXISTS sop_procedures (
			id TEXT PRIMARY KEY,
			node_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_sop_procedures_node_id ON sop_procedures(node_id);
	`

	dropSOPProcedureTablesSQL = `DROP TABLE IF EXISTS sop_procedures CASCADE`
)

func (c *Client) CreateSOPProcedure(id, nodeID, title, content string) error {
	insertSQL := `
		INSERT INTO sop_procedures (id, node_id, title, content)
		VALUES ($1, $2, $3, $4)
	`

	if _, err := c.db.Exec(insertSQL, id, nodeID, title, content); err != nil {
		return fmt.Errorf("failed to create SOP procedure: %w", err)
	}

	return nil
}

func (c *Client) GetSOPProcedure(id string) (*servicev1.SOPProcedure, error) {
	querySQL := `
		SELECT id, node_id, title, content, created_at, updated_at
		FROM sop_procedures
		WHERE id = $1
	`

	var proc servicev1.SOPProcedure
	var nodeID, title, content sql.NullString
	var createdAt, updatedAt time.Time

	if err := c.db.QueryRow(querySQL, id).Scan(
		&proc.Id,
		&nodeID,
		&title,
		&content,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get SOP procedure: %w", err)
	}

	if nodeID.Valid {
		proc.NodeId = nodeID.String
	}
	if title.Valid {
		proc.Title = title.String
	}
	if content.Valid {
		proc.Content = content.String
	}

	proc.CreatedAt = timestampToProto(createdAt)
	proc.UpdatedAt = timestampToProto(updatedAt)
	return &proc, nil
}

func (c *Client) ListSOPProceduresByNodeID(nodeID string) ([]*servicev1.SOPProcedure, error) {
	querySQL := `
		SELECT id, node_id, title, content, created_at, updated_at
		FROM sop_procedures
		WHERE node_id = $1
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(querySQL, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list SOP procedures: %w", err)
	}
	defer rows.Close()

	var procedures []*servicev1.SOPProcedure
	for rows.Next() {
		var proc servicev1.SOPProcedure
		var dbNodeID, title, content sql.NullString
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&proc.Id,
			&dbNodeID,
			&title,
			&content,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan SOP procedure: %w", err)
		}

		if dbNodeID.Valid {
			proc.NodeId = dbNodeID.String
		}
		if title.Valid {
			proc.Title = title.String
		}
		if content.Valid {
			proc.Content = content.String
		}

		proc.CreatedAt = timestampToProto(createdAt)
		proc.UpdatedAt = timestampToProto(updatedAt)
		procedures = append(procedures, &proc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating SOP procedures: %w", err)
	}

	return procedures, nil
}

func (c *Client) UpdateSOPProcedure(id, title, content string) error {
	updateSQL := `
		UPDATE sop_procedures
		SET title = $1, content = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`

	if _, err := c.db.Exec(updateSQL, title, content, id); err != nil {
		return fmt.Errorf("failed to update SOP procedure: %w", err)
	}

	return nil
}

func (c *Client) DeleteSOPProcedure(id string) error {
	deleteSQL := `DELETE FROM sop_procedures WHERE id = $1`

	if _, err := c.db.Exec(deleteSQL, id); err != nil {
		return fmt.Errorf("failed to delete SOP procedure: %w", err)
	}

	return nil
}
