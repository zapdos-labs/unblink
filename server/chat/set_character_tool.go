package chat

import (
	"context"
	"encoding/json"
	"fmt"
)

type contextKey string

const conversationIDKey contextKey = "conversationID"

type SetCharacterTool struct {
	db Database
}

func (t *SetCharacterTool) Id() string {
	return "set_character"
}

func (t *SetCharacterTool) Name() string {
	return "Set character"
}

func (t *SetCharacterTool) Description() string {
	return "Set the assistant character/system prompt for this conversation."
}

func (t *SetCharacterTool) Parameters() map[string]any {
	return map[string]any{
		"character": map[string]any{
			"type":        "string",
			"description": "New character/system prompt text for this conversation",
		},
	}
}

func (t *SetCharacterTool) DisplayMessage(argumentsJSON string) string {
	var args struct {
		Character string `json:"character"`
	}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return "Updating character"
	}
	return "Updating character"
}

func (t *SetCharacterTool) Execute(ctx context.Context, argumentsJSON string) string {
	conversationID, ok := ctx.Value(conversationIDKey).(string)
	if !ok || conversationID == "" {
		return `{"error":"conversation id missing"}`
	}

	var args struct {
		Character string `json:"character"`
	}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments: %s"}`, err)
	}

	if args.Character == "" {
		return `{"error":"character is required"}`
	}

	if len(args.Character) > 10000 {
		return `{"error":"character too long (max 10000 characters)"}`
	}

	if err := t.db.SetSystemPrompt(conversationID, args.Character); err != nil {
		return fmt.Sprintf(`{"error":"failed to set character: %s"}`, err)
	}

	return `{"success":true}`
}

func RegisterSetCharacterTool(s *Service) {
	s.RegisterTool(&SetCharacterTool{db: s.db})
}
