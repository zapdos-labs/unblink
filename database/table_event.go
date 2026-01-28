package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	servicev1 "unblink/server/gen/service/v1"

	"google.golang.org/protobuf/types/known/structpb"
)

const (
	createEventTablesSQL = `
		CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			service_id TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_events_service_id ON events(service_id);
		CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
	`

	dropEventTablesSQL = `DROP TABLE IF EXISTS events CASCADE`
)

// CreateEvent creates a new event
func (c *Client) CreateEvent(id, serviceID string, payload *structpb.Value) error {
	payloadJSON, err := protoValueToJSON(payload)
	if err != nil {
		return fmt.Errorf("failed to convert payload: %w", err)
	}

	insertSQL := `
		INSERT INTO events (id, service_id, payload)
		VALUES ($1, $2, $3::jsonb)
	`

	_, err = c.db.Exec(insertSQL, id, serviceID, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

// GetEvent retrieves an event by ID
func (c *Client) GetEvent(id string) (*servicev1.Event, error) {
	querySQL := `
		SELECT id, service_id, payload, created_at
		FROM events
		WHERE id = $1
	`

	var event servicev1.Event
	var serviceID sql.NullString
	var payloadJSON sql.NullString
	var createdAt time.Time

	err := c.db.QueryRow(querySQL, id).Scan(
		&event.Id,
		&serviceID,
		&payloadJSON,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	if serviceID.Valid {
		event.ServiceId = serviceID.String
	}
	if payloadJSON.Valid {
		payload, err := jsonToProtoValue(payloadJSON.String)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload: %w", err)
		}
		event.Payload = payload
	}

	event.CreatedAt = timestampToProto(createdAt)

	return &event, nil
}

// DeleteEvent removes an event by ID
func (c *Client) DeleteEvent(id string) error {
	deleteSQL := `DELETE FROM events WHERE id = $1`

	_, err := c.db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// ListEventsByServiceId retrieves all events for a service
func (c *Client) ListEventsByServiceId(serviceID string) ([]*servicev1.Event, error) {
	querySQL := `
		SELECT id, service_id, payload, created_at
		FROM events
		WHERE service_id = $1
		ORDER BY created_at DESC
	`

	rows, err := c.db.Query(querySQL, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []*servicev1.Event

	for rows.Next() {
		var event servicev1.Event
		var svcID sql.NullString
		var payloadJSON sql.NullString
		var createdAt time.Time

		if err := rows.Scan(
			&event.Id,
			&svcID,
			&payloadJSON,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if svcID.Valid {
			event.ServiceId = svcID.String
		}
		if payloadJSON.Valid {
			payload, err := jsonToProtoValue(payloadJSON.String)
			if err != nil {
				return nil, fmt.Errorf("failed to convert payload: %w", err)
			}
			event.Payload = payload
		}

		event.CreatedAt = timestampToProto(createdAt)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// protoValueToJSON converts a protobuf Value to JSON string
func protoValueToJSON(v *structpb.Value) (string, error) {
	if v == nil {
		return "null", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// jsonToProtoValue converts a JSON string to protobuf Value
func jsonToProtoValue(s string) (*structpb.Value, error) {
	var v structpb.Value
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return &v, nil
}
