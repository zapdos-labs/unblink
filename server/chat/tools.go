package chat

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
)

// Tool defines the interface for tools that can be executed by the chat service
type Tool interface {
	// Name returns the tool name (unique identifier)
	Name() string
	// Description returns a description of what the tool does
	Description() string
	// Parameters returns the JSON schema for the tool parameters
	Parameters() map[string]any
	// Execute executes the tool with the given arguments
	Execute(ctx context.Context, argumentsJSON string) string
}

// Displayable is an optional interface for tools that want to provide a custom display message
type Displayable interface {
	Tool
	// DisplayMessage returns a human-friendly message describing what the tool is doing
	// argumentsJSON contains the tool arguments as a JSON string
	DisplayMessage(argumentsJSON string) string
}

// GetDisplayMessage returns the display message for a tool, falling back to a default format
func GetDisplayMessage(tool Tool, argumentsJSON string) string {
	if d, ok := tool.(Displayable); ok {
		return d.DisplayMessage(argumentsJSON)
	}
	return fmt.Sprintf("Running %s", tool.Name())
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// Execute executes a tool by name
func (r *ToolRegistry) Execute(ctx context.Context, name string, argumentsJSON string) string {
	if tool, ok := r.Get(name); ok {
		return tool.Execute(ctx, argumentsJSON)
	}
	return ""
}

// AsOpenAITools converts the registry to OpenAI tool definitions
func (r *ToolRegistry) AsOpenAITools() []openai.ChatCompletionToolUnionParam {
	var result []openai.ChatCompletionToolUnionParam
	for _, tool := range r.tools {
		result = append(result, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name(),
			Description: openai.String(tool.Description()),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": tool.Parameters(),
			},
		}))
	}
	return result
}
