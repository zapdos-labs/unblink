package models

import (
	"log"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

// EstimateTokens estimates token count for text using character-based counting.
// Conservative estimate: ~3 characters per token (accounts for markdown and code).
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Conservative: 3 chars per token
	return len(text) / 3
}

// EstimateMessageTokens estimates tokens for a chat message
func EstimateMessageTokens(msg openai.ChatCompletionMessageParamUnion) int {
	// Base tokens for role/formatting overhead
	baseTokens := 4

	switch {
	case msg.OfUser != nil:
		// User message with content
		tokens := baseTokens + estimateUserContentTokens(msg.OfUser.Content)
		// log.Printf("[models] User message: %d tokens", tokens)
		return tokens

	case msg.OfSystem != nil:
		// System message with content
		tokens := baseTokens + estimateSystemContentTokens(msg.OfSystem.Content)
		// log.Printf("[models] System message: %d tokens", tokens)
		return tokens

	case msg.OfAssistant != nil:
		// Assistant message
		tokens := baseTokens
		tokens += estimateAssistantContentTokens(msg.OfAssistant.Content)
		// Tool calls add tokens
		for _, tc := range msg.OfAssistant.ToolCalls {
			tokens += estimateToolCallTokens(tc)
		}
		// log.Printf("[models] Assistant message: %d tokens", tokens)
		return tokens

	case msg.OfTool != nil:
		// Tool message with content
		tokens := baseTokens + estimateToolContentTokens(msg.OfTool.Content)
		log.Printf("[models] Tool message: %d tokens", tokens)
		return tokens

	case msg.OfDeveloper != nil:
		// Developer message with content
		tokens := baseTokens + estimateDeveloperContentTokens(msg.OfDeveloper.Content)
		// log.Printf("[models] Developer message: %d tokens", tokens)
		return tokens

	default:
		// log.Printf("[models] Unknown message type: %d base tokens", baseTokens)
		return baseTokens
	}
}

// estimateUserContentTokens extracts tokens from user content union
func estimateUserContentTokens(content openai.ChatCompletionUserMessageParamContentUnion) int {
	if !param.IsOmitted(content.OfString) {
		return EstimateTokens(content.OfString.Value)
	}
	// Array of content parts - approximate
	if len(content.OfArrayOfContentParts) > 0 {
		return 500 // Rough estimate for complex content
	}
	return 0
}

// estimateSystemContentTokens extracts tokens from system content union
func estimateSystemContentTokens(content openai.ChatCompletionSystemMessageParamContentUnion) int {
	if !param.IsOmitted(content.OfString) {
		return EstimateTokens(content.OfString.Value)
	}
	if len(content.OfArrayOfContentParts) > 0 {
		return 500
	}
	return 0
}

// estimateAssistantContentTokens extracts tokens from assistant content union
func estimateAssistantContentTokens(content openai.ChatCompletionAssistantMessageParamContentUnion) int {
	if !param.IsOmitted(content.OfString) {
		return EstimateTokens(content.OfString.Value)
	}
	if len(content.OfArrayOfContentParts) > 0 {
		return 500
	}
	return 0
}

// estimateDeveloperContentTokens extracts tokens from developer content union
func estimateDeveloperContentTokens(content openai.ChatCompletionDeveloperMessageParamContentUnion) int {
	if !param.IsOmitted(content.OfString) {
		return EstimateTokens(content.OfString.Value)
	}
	if len(content.OfArrayOfContentParts) > 0 {
		return 500
	}
	return 0
}

// estimateToolContentTokens extracts tokens from tool content union
func estimateToolContentTokens(content openai.ChatCompletionToolMessageParamContentUnion) int {
	if !param.IsOmitted(content.OfString) {
		return EstimateTokens(content.OfString.Value)
	}
	// Array of content parts - approximate
	if len(content.OfArrayOfContentParts) > 0 {
		return 500
	}
	return 0
}

// estimateToolCallTokens estimates tokens for a tool call
func estimateToolCallTokens(tc openai.ChatCompletionMessageToolCallUnionParam) int {
	tokens := 10 // Base tokens for tool call structure
	if tc.OfFunction != nil {
		tokens += EstimateTokens(tc.OfFunction.Function.Name)
		tokens += EstimateTokens(tc.OfFunction.Function.Arguments)
	}
	return tokens
}

// EstimateConversationTokens estimates total tokens for a conversation
func EstimateConversationTokens(messages []openai.ChatCompletionMessageParamUnion) int {
	total := 0
	for _, msg := range messages {
		total += EstimateMessageTokens(msg)
	}
	return total
}
