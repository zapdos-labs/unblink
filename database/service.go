package database

import (
	"database/sql"
	"fmt"
	"time"

	servicev1 "unblink/server/gen/service/v1"
)

const (
	createServiceTablesSQL = `
		CREATE TABLE IF NOT EXISTS services (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			node_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_services_node_id ON services(node_id);
	`

	dropServiceTablesSQL = `DROP TABLE IF EXISTS services CASCADE`

	createUserNodeTablesSQL = `
		CREATE TABLE IF NOT EXISTS user_node (
			user_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, node_id)
		);
		CREATE INDEX IF NOT EXISTS idx_user_node_node_id ON user_node(node_id);
	`

	dropUserNodeTablesSQL = `DROP TABLE IF EXISTS user_node CASCADE`

	createStorageTablesSQL = `
		CREATE TABLE IF NOT EXISTS storage (
			id TEXT PRIMARY KEY,
			service_id TEXT NOT NULL,
			type TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			file_size BIGINT,
			content_type TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			metadata TEXT,
			FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_storage_service_id ON storage(service_id);
		CREATE INDEX IF NOT EXISTS idx_storage_type ON storage(type);
		CREATE INDEX IF NOT EXISTS idx_storage_timestamp ON storage(timestamp);
		CREATE INDEX IF NOT EXISTS idx_storage_service_type ON storage(service_id, type);
		CREATE INDEX IF NOT EXISTS idx_storage_service_timestamp ON storage(service_id, timestamp DESC);
	`

	dropStorageTablesSQL = `DROP TABLE IF EXISTS storage CASCADE`
)

// CreateService creates a new service
func (c *Client) CreateService(id, name, url, nodeID string) error {
	insertSQL := `
		INSERT INTO services (id, name, url, node_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			url = EXCLUDED.url,
			node_id = EXCLUDED.node_id,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := c.db.Exec(insertSQL, id, name, url, nodeID)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	return nil
}

// UpdateService updates an existing service
func (c *Client) UpdateService(id, name, url, userID string) error {
	updateSQL := `
		UPDATE services
		SET name = $1, url = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		AND (
			-- Node is public (no users associated)
			NOT EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = services.node_id
			)
			OR
			-- OR user has access to this private node
			EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = services.node_id
				AND user_node.user_id = $4
			)
		)
	`

	result, err := c.db.Exec(updateSQL, name, url, id, userID)
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetService retrieves a service by ID (no authorization check - use with DeleteService)
func (c *Client) GetService(id string) (*servicev1.Service, error) {
	querySQL := `
		SELECT s.id, s.name, s.url, s.node_id, s.created_at, s.updated_at
		FROM services s
		WHERE s.id = $1
	`

	var svc servicev1.Service
	var name, url sql.NullString
	var svcNodeID sql.NullString
	var createdAt, updatedAt time.Time

	err := c.db.QueryRow(querySQL, id).Scan(
		&svc.Id,
		&name,
		&url,
		&svcNodeID,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	if name.Valid {
		svc.Name = name.String
	}
	if url.Valid {
		svc.Url = url.String
	}
	if svcNodeID.Valid {
		svc.NodeId = svcNodeID.String
	}

	svc.CreatedAt = timestampToProto(createdAt)
	svc.UpdatedAt = timestampToProto(updatedAt)

	return &svc, nil
}

// DeleteService removes a service by ID with public/private logic
func (c *Client) DeleteService(id, userID string) error {
	deleteSQL := `
		DELETE FROM services
		WHERE id = $1
		AND (
			-- Node is public (no users associated)
			NOT EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = services.node_id
			)
			OR
			-- OR user has access to this private node
			EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = services.node_id
				AND user_node.user_id = $2
			)
		)
	`

	result, err := c.db.Exec(deleteSQL, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListServicesByNodeId retrieves all services for a node with public/private logic
func (c *Client) ListServicesByNodeId(nodeID, userID string) ([]*servicev1.Service, error) {
	querySQL := `
		SELECT s.id, s.name, s.url, s.node_id, s.created_at, s.updated_at
		FROM services s
		WHERE s.node_id = $1
		AND (
			-- Node is public (no users associated)
			NOT EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = s.node_id
			)
			OR
			-- OR user has access to this private node
			EXISTS (
				SELECT 1 FROM user_node
				WHERE user_node.node_id = s.node_id
				AND user_node.user_id = $2
			)
		)
		ORDER BY s.created_at DESC
	`

	rows, err := c.db.Query(querySQL, nodeID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	defer rows.Close()

	var services []*servicev1.Service

	for rows.Next() {
		var svc servicev1.Service
		var name, url, svcNodeID sql.NullString
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&svc.Id,
			&name,
			&url,
			&svcNodeID,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}

		if name.Valid {
			svc.Name = name.String
		}
		if url.Valid {
			svc.Url = url.String
		}
		if svcNodeID.Valid {
			svc.NodeId = svcNodeID.String
		}

		svc.CreatedAt = timestampToProto(createdAt)
		svc.UpdatedAt = timestampToProto(updatedAt)

		services = append(services, &svc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}

// ListAllServices retrieves all services for registry initialization
func (c *Client) ListAllServices() ([]*servicev1.Service, error) {
	querySQL := `
		SELECT s.id, s.name, s.url, s.node_id, s.created_at, s.updated_at
		FROM services s
		ORDER BY s.created_at DESC
	`

	rows, err := c.db.Query(querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to list all services: %w", err)
	}
	defer rows.Close()

	var services []*servicev1.Service

	for rows.Next() {
		var svc servicev1.Service
		var name, url, svcNodeID sql.NullString
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&svc.Id,
			&name,
			&url,
			&svcNodeID,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan service: %w", err)
		}

		if name.Valid {
			svc.Name = name.String
		}
		if url.Valid {
			svc.Url = url.String
		}
		if svcNodeID.Valid {
			svc.NodeId = svcNodeID.String
		}

		svc.CreatedAt = timestampToProto(createdAt)
		svc.UpdatedAt = timestampToProto(updatedAt)

		services = append(services, &svc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating services: %w", err)
	}

	return services, nil
}
