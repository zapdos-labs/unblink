package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"unblink/database"
)

// VideoSearchTool is a tool for searching videos
type VideoSearchTool struct {
	db *database.Client
}

// NewVideoSearchTool creates a new video search tool
func NewVideoSearchTool(db *database.Client) *VideoSearchTool {
	return &VideoSearchTool{
		db: db,
	}
}

// Name returns the tool name
func (t *VideoSearchTool) Name() string {
	return "search_video_events"
}

// Description returns the tool description
func (t *VideoSearchTool) Description() string {
	return "Search video events of the users. Returns matching VLM-indexed video events from user cameras with summaries and metadata. Only return results from this tool - do not hallucinate."
}

// Parameters returns the JSON schema for tool parameters
func (t *VideoSearchTool) Parameters() map[string]any {
	return map[string]any{
		"keywords": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "List of keywords to search for in video event (example keywords: person, car, dog, cat, bicycle, etc.)",
		},
	}
}

// Execute executes the tool with the given arguments
func (t *VideoSearchTool) Execute(ctx context.Context, argumentsJSON string) string {
	var args struct {
		Keywords []string `json:"keywords"`
	}

	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Name(), fmt.Sprintf(`{"error": "invalid arguments: %v"}`, err))
	}

	if len(args.Keywords) == 0 {
		return fmt.Sprintf("tool %s returned: %s", t.Name(), `{"error": "keywords must be provided"}`)
	}

	// Search events by keywords
	events, err := t.db.SearchEventsByPayload(args.Keywords)
	if err != nil {
		return fmt.Sprintf("tool %s returned: %s", t.Name(), fmt.Sprintf(`{"error": "search failed: %v"}`, err))
	}

	if len(events) == 0 {
		return fmt.Sprintf("tool %s returned: %s", t.Name(), `{"result": "There is no such video in the database."}`)
	}

	// Convert events to JSON response
	type eventResult struct {
		ID        string         `json:"id"`
		ServiceID string         `json:"service_id"`
		Payload   map[string]any `json:"payload"`
		CreatedAt string         `json:"created_at"`
	}

	results := make([]eventResult, len(events))
	for i, e := range events {
		results[i] = eventResult{
			ID:        e.Id,
			ServiceID: e.ServiceId,
			Payload:   e.Payload.AsMap(),
			CreatedAt: e.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z"),
		}
	}

	responseJSON, _ := json.Marshal(map[string]any{
		"result": fmt.Sprintf("Found %d matching video(s)", len(events)),
		"videos": results,
		"count":  len(events),
	})

	return fmt.Sprintf("tool %s returned: %s", t.Name(), string(responseJSON))
}

// DisplayMessage returns a human-friendly message describing what the tool is doing
func (t *VideoSearchTool) DisplayMessage(argumentsJSON string) string {
	var args struct {
		Keywords []string `json:"keywords"`
	}
	json.Unmarshal([]byte(argumentsJSON), &args)
	return fmt.Sprintf("Searching videos for: %s", strings.Join(args.Keywords, ", "))
}
