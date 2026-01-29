package tools

import (
	"context"
	"encoding/json"

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
	return "video_search"
}

// Description returns the tool description
func (t *VideoSearchTool) Description() string {
	return "Search for videos based on a query. You must not hallucinate results. If no videos match the query, respond with 'There is no such video in the database.' Only use information returned by this tool."
}

// Parameters returns the JSON schema for tool parameters
func (t *VideoSearchTool) Parameters() map[string]any {
	return map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "The search query to find videos",
		},
	}
}

// Execute executes the tool with the given arguments
func (t *VideoSearchTool) Execute(ctx context.Context, argumentsJSON string) string {
	var args struct {
		Query string `json:"query"`
	}

	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return `Invalid arguments provided.`
	}

	// TODO: Query database using t.db
	// For now, always return "no such video"
	return `Tool video_search returned: There is no such video in the database. Tell user so.`
}
