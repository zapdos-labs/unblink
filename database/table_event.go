package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"

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
		CREATE INDEX IF NOT EXISTS idx_events_payload_gin ON events USING gin(payload);
	`

	dropEventTablesSQL = `DROP TABLE IF EXISTS events CASCADE`
)

// CreateEvent creates a new event
func (c *Client) CreateEvent(id, serviceID string, payload *structpb.Struct) error {
	payloadJSON, err := protoStructToJSON(payload)
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
		payload, err := jsonToProtoStruct(payloadJSON.String)
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

// ListEventsByNodeId retrieves events for all services in a node with pagination
func (c *Client) ListEventsByNodeId(nodeID string, pageSize, pageOffset int32) ([]*servicev1.Event, int32, error) {
	// Set defaults
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	if pageOffset < 0 {
		pageOffset = 0
	}

	// Get total count
	var totalCount int32
	countSQL := `
		SELECT COUNT(*)
		FROM events e
		JOIN services s ON e.service_id = s.id
		WHERE s.node_id = $1
	`
	err := c.db.QueryRow(countSQL, nodeID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count events: %w", err)
	}

	// Get paginated results
	querySQL := `
		SELECT e.id, e.service_id, e.payload, e.created_at
		FROM events e
		JOIN services s ON e.service_id = s.id
		WHERE s.node_id = $1
		ORDER BY e.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := c.db.Query(querySQL, nodeID, pageSize, pageOffset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list events: %w", err)
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
			return nil, 0, fmt.Errorf("failed to scan event: %w", err)
		}

		if svcID.Valid {
			event.ServiceId = svcID.String
		}
		if payloadJSON.Valid {
			payload, err := jsonToProtoStruct(payloadJSON.String)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to convert payload: %w", err)
			}
			event.Payload = payload
		}

		event.CreatedAt = timestampToProto(createdAt)

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating events: %w", err)
	}

	return events, totalCount, nil
}

// ListVLMEventsByGranularityRange returns VLM-indexing events for a service and granularity
// that overlap with [from, to), ordered by start time.
func (c *Client) ListVLMEventsByGranularityRange(serviceID, granularity string, from, to time.Time) ([]*servicev1.Event, error) {
	startExpr := `COALESCE(NULLIF(e.payload->>'from_iso', '')::timestamptz, e.created_at AT TIME ZONE 'UTC')`
	endExpr := `COALESCE(NULLIF(e.payload->>'to_iso', '')::timestamptz, e.created_at AT TIME ZONE 'UTC')`

	querySQL := fmt.Sprintf(`
		SELECT e.id, e.service_id, e.payload, e.created_at
		FROM events e
		WHERE e.service_id = $1
		  AND e.payload->>'type' = 'vlm-indexing'
		  AND e.payload->>'granularity' = $2
		  AND %s > $3
		  AND %s < $4
		ORDER BY %s ASC, e.created_at ASC
	`, endExpr, startExpr, startExpr)

	rows, err := c.db.Query(querySQL, serviceID, granularity, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to list vlm events by granularity range: %w", err)
	}
	defer rows.Close()

	var events []*servicev1.Event
	for rows.Next() {
		var event servicev1.Event
		var svcID sql.NullString
		var payloadJSON sql.NullString
		var createdAt time.Time

		if err := rows.Scan(&event.Id, &svcID, &payloadJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if svcID.Valid {
			event.ServiceId = svcID.String
		}
		if payloadJSON.Valid {
			payload, err := jsonToProtoStruct(payloadJSON.String)
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

// protoStructToJSON converts a protobuf Struct to JSON string
// It converts the protobuf Struct to a native Go map first to get clean JSON
func protoStructToJSON(s *structpb.Struct) (string, error) {
	if s == nil {
		return "{}", nil
	}
	// Convert to native Go map to get clean JSON
	nativeMap := s.AsMap()
	b, err := json.Marshal(nativeMap)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// jsonToProtoStruct converts a JSON string to protobuf Struct
// It first unmarshals to a native Go map, then converts to protobuf Struct
func jsonToProtoStruct(s string) (*structpb.Struct, error) {
	// First unmarshal to native Go map
	var nativeMap map[string]interface{}
	if err := json.Unmarshal([]byte(s), &nativeMap); err != nil {
		return nil, err
	}
	// Convert native Go map to protobuf Struct
	return structpb.NewStruct(nativeMap)
}

// CountEventsForUser counts all events accessible to a user.
// A user can access events from:
// - Services on public nodes (no users associated)
// - Services on private nodes where the user is explicitly associated
func (c *Client) CountEventsForUser(userID string) (int64, error) {
	querySQL := `
		SELECT COUNT(DISTINCT e.id)
		FROM events e
		JOIN services s ON e.service_id = s.id
		LEFT JOIN user_node un ON s.node_id = un.node_id
		WHERE un.node_id IS NULL OR un.user_id = $1
	`

	var count int64
	err := c.db.QueryRow(querySQL, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events for user: %w", err)
	}

	return count, nil
}

// CameraEventQuery defines the supported filters for querying accessible camera events.
type CameraEventQuery struct {
	SearchTerms []string
	From        *time.Time
	To          *time.Time
	Granularity string
	ServiceID   string
	Limit       int
}

// QueryCameraEventsForUser queries VLM-indexed events visible to the user.
// Search terms are matched against the structured VLM description and object labels.
func (c *Client) QueryCameraEventsForUser(userID string, query CameraEventQuery) ([]*servicev1.Event, error) {
	const (
		defaultLimit = 20
		maxLimit     = 100
	)

	limit := query.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	startExpr := `COALESCE(NULLIF(e.payload->>'from_iso', '')::timestamptz, e.created_at AT TIME ZONE 'UTC')`
	endExpr := `COALESCE(NULLIF(e.payload->>'to_iso', '')::timestamptz, e.created_at AT TIME ZONE 'UTC')`

	args := make([]interface{}, 0, len(query.SearchTerms)+6)
	args = append(args, userID)

	whereClauses := []string{
		`e.payload->>'type' = 'vlm-indexing'`,
		`(
			NOT EXISTS (
				SELECT 1
				FROM user_node un
				WHERE un.node_id = s.node_id
			)
			OR EXISTS (
				SELECT 1
				FROM user_node un
				WHERE un.node_id = s.node_id AND un.user_id = $1
			)
		)`,
	}

	if query.ServiceID != "" {
		args = append(args, query.ServiceID)
		whereClauses = append(whereClauses, fmt.Sprintf("e.service_id = $%d", len(args)))
	}

	if query.Granularity != "" {
		args = append(args, query.Granularity)
		whereClauses = append(whereClauses, fmt.Sprintf(`e.payload->>'granularity' = $%d`, len(args)))
	}

	if query.From != nil {
		args = append(args, query.From.UTC())
		whereClauses = append(whereClauses, fmt.Sprintf("%s >= $%d", endExpr, len(args)))
	}

	if query.To != nil {
		args = append(args, query.To.UTC())
		whereClauses = append(whereClauses, fmt.Sprintf("%s < $%d", startExpr, len(args)))
	}

	orderBy := fmt.Sprintf("%s DESC, e.created_at DESC", endExpr)
	if len(query.SearchTerms) > 0 {
		searchClauses := make([]string, 0, len(query.SearchTerms))
		scoreClauses := make([]string, 0, len(query.SearchTerms))
		for _, term := range query.SearchTerms {
			args = append(args, "%"+strings.ToLower(term)+"%")
			placeholder := fmt.Sprintf("$%d", len(args))

			matchExpr := fmt.Sprintf(`(
				COALESCE(LOWER(e.payload->'response'->>'description'), '') LIKE %s
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(COALESCE(e.payload->'response'->'objects', '[]'::jsonb)) AS obj
					WHERE COALESCE(LOWER(obj->>'label'), '') LIKE %s
				)
			)`, placeholder, placeholder)

			searchClauses = append(searchClauses, matchExpr)
			scoreClauses = append(scoreClauses, fmt.Sprintf("CASE WHEN %s THEN 1 ELSE 0 END", matchExpr))
		}

		whereClauses = append(whereClauses, fmt.Sprintf("(%s)", strings.Join(searchClauses, " OR ")))
		orderBy = fmt.Sprintf("(%s) DESC, %s DESC, e.created_at DESC", strings.Join(scoreClauses, " + "), endExpr)
	}

	args = append(args, limit)
	querySQL := fmt.Sprintf(`
		SELECT e.id, e.service_id, e.payload, e.created_at
		FROM events e
		JOIN services s ON e.service_id = s.id
		WHERE %s
		ORDER BY %s
		LIMIT $%d
	`, strings.Join(whereClauses, "\n\t\tAND "), orderBy, len(args))

	rows, err := c.db.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
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
			payload, err := jsonToProtoStruct(payloadJSON.String)
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
