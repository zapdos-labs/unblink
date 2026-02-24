package models

import (
	"fmt"
	"log"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

// Trimmer trims conversation history to fit within model context limits
type Trimmer struct {
	maxTokens    int
	safetyMargin int // Percentage (0-100)
}

// NewTrimmer creates a new content trimmer
func NewTrimmer(maxTokens, safetyMargin int) *Trimmer {
	return &Trimmer{
		maxTokens:    maxTokens,
		safetyMargin: safetyMargin,
	}
}

// TrimToFit trims messages to fit within the model's context window using greedy tail-to-head algorithm
func (t *Trimmer) TrimToFit(messages []openai.ChatCompletionMessageParamUnion) ([]openai.ChatCompletionMessageParamUnion, error) {
	currentTokens := EstimateConversationTokens(messages)
	availableTokens := t.maxTokens - (t.maxTokens*t.safetyMargin)/100 - 2000 // Reserve 2000 for response

	log.Printf("[models.Trimmer] Estimated tokens: %d, available: %d (max=%d, margin=%d%%)",
		currentTokens, availableTokens, t.maxTokens, t.safetyMargin)

	// Log input messages for debugging
	logMessagesSummary("[models.Trimmer] Input:", messages)

	// Separate system messages from conversation
	var systemMsgs []openai.ChatCompletionMessageParamUnion
	for _, msg := range messages {
		if msg.OfSystem != nil {
			systemMsgs = append(systemMsgs, msg)
		}
	}

	// Budget for conversation (excluding system prompts)
	budget := availableTokens - EstimateConversationTokens(systemMsgs)
	if budget < 0 {
		budget = 0
	}

	// Collect conversation messages (non-system) in reverse order (prepend to get oldest-first)
	var conversation []openai.ChatCompletionMessageParamUnion
	added := make(map[int]bool)

	// Iterate from last message to first (backwards in time)
	for i := len(messages) - 1; i >= 0; i-- {
		if added[i] {
			continue
		}

		msg := messages[i]

		// Skip system messages
		if msg.OfSystem != nil {
			added[i] = true
			continue
		}

		// Check if this is a tool message
		if msg.OfTool != nil {
			// Find the assistant message that called this tool
			callerIdx, callerMsg := findCallerWithToolCallID(messages, i, msg.OfTool.ToolCallID)

			if callerIdx == -1 {
				log.Printf("[models.Trimmer] Orphaned tool message at index %d, skipping", i)
				added[i] = true
				continue
			}

			// Calculate cost: assistant + tool message + everything between
			costBetween := 0
			for j := callerIdx + 1; j < i; j++ {
				if !added[j] {
					costBetween += EstimateMessageTokens(messages[j])
				}
			}

			cost := EstimateMessageTokens(callerMsg) + costBetween + EstimateMessageTokens(msg)

			if cost <= budget {
				// Prepend assistant message (will end up in correct order)
				conversation = append([]openai.ChatCompletionMessageParamUnion{callerMsg}, conversation...)
				added[callerIdx] = true

				// Prepend any messages between
				for j := i - 1; j > callerIdx; j-- {
					if !added[j] {
						conversation = append([]openai.ChatCompletionMessageParamUnion{messages[j]}, conversation...)
						added[j] = true
					}
				}

				// Prepend tool message
				conversation = append([]openai.ChatCompletionMessageParamUnion{msg}, conversation...)
				added[i] = true
				budget -= cost
				log.Printf("[models.Trimmer] Added tool+assistant pair at indices %d,%d (cost=%d, remaining budget=%d)",
					callerIdx, i, cost, budget)
			} else {
				// Over budget, try to trim tool content
				remainingBudget := budget - EstimateMessageTokens(callerMsg)
				if remainingBudget < 500 {
					log.Printf("[models.Trimmer] Cannot fit tool+assistant pair (need %d, have %d), stopping",
						cost, budget)
					break
				}

				content := extractToolContent(msg.OfTool.Content)
				truncated := truncateToolContentMaxTokens(content, remainingBudget)
				if truncated == "" {
					log.Printf("[models.Trimmer] Cannot trim tool content sufficiently, stopping")
					break
				}

				// Prepend assistant with truncated tool message
				truncatedMsg := openai.ToolMessage(truncated, msg.OfTool.ToolCallID)
				conversation = append([]openai.ChatCompletionMessageParamUnion{truncatedMsg}, conversation...)
				conversation = append([]openai.ChatCompletionMessageParamUnion{callerMsg}, conversation...)
				added[callerIdx] = true
				added[i] = true
				budget = 0
				log.Printf("[models.Trimmer] Added assistant with truncated tool message (remaining budget=0)")
				break
			}
			continue
		}

		// Not a tool message - check if it fits
		cost := EstimateMessageTokens(msg)
		if cost <= budget {
			conversation = append([]openai.ChatCompletionMessageParamUnion{msg}, conversation...)
			added[i] = true
			budget -= cost
			log.Printf("[models.Trimmer] Added message at index %d (cost=%d, remaining budget=%d)", i, cost, budget)
		} else {
			log.Printf("[models.Trimmer] Cannot fit message at index %d (need %d, have %d), stopping",
				i, cost, budget)
			break
		}
	}

	// Combine: system messages first, then conversation
	result := append(systemMsgs, conversation...)

	log.Printf("[models.Trimmer] Final result: %d messages (was %d)", len(result), len(messages))
	logMessagesSummary("[models.Trimmer] Result:", result)
	return result, nil
}

// logMessagesSummary logs a concise summary of messages
func logMessagesSummary(prefix string, messages []openai.ChatCompletionMessageParamUnion) {
	for i, msg := range messages {
		role := "unknown"
		preview := ""

		// Check user first since user messages have content that might look like assistant text
		switch {
		case msg.OfUser != nil:
			role = "user"
			if !param.IsOmitted(msg.OfUser.Content.OfString) {
				preview = truncatePreview(msg.OfUser.Content.OfString.Value)
			}
		case msg.OfSystem != nil:
			role = "system"
			if !param.IsOmitted(msg.OfSystem.Content.OfString) {
				preview = truncatePreview(msg.OfSystem.Content.OfString.Value)
			}
		case msg.OfAssistant != nil:
			role = "assistant"
			if !param.IsOmitted(msg.OfAssistant.Content.OfString) {
				preview = truncatePreview(msg.OfAssistant.Content.OfString.Value)
			}
			if len(msg.OfAssistant.ToolCalls) > 0 {
				preview = fmt.Sprintf("[%d tool calls]", len(msg.OfAssistant.ToolCalls))
			}
		case msg.OfTool != nil:
			role = "tool"
			if !param.IsOmitted(msg.OfTool.Content.OfString) {
				preview = truncatePreview(msg.OfTool.Content.OfString.Value)
			}
		case msg.OfDeveloper != nil:
			role = "developer"
			if !param.IsOmitted(msg.OfDeveloper.Content.OfString) {
				preview = truncatePreview(msg.OfDeveloper.Content.OfString.Value)
			}
		}

		log.Printf("%s [%d] %s: %s", prefix, i, role, preview)
	}
}

// truncatePreview truncates text to 20 chars for preview
func truncatePreview(s string) string {
	if len(s) <= 20 {
		return s
	}
	return s[:20] + "..."
}

// extractToolContent extracts string content from tool content union
func extractToolContent(content openai.ChatCompletionToolMessageParamContentUnion) string {
	if !param.IsOmitted(content.OfString) {
		return content.OfString.Value
	}
	return ""
}

// truncateToolContent truncates content to fit within token limit
func truncateToolContent(content string, maxTokens int) string {
	maxChars := maxTokens * 3
	if len(content) <= maxChars {
		return content
	}

	// Check if it's news tool output
	if strings.Contains(content, "## ") && strings.Contains(content, "**Source:**") {
		return truncateNewsContent(content, maxChars)
	}

	return content[:maxChars] + "\n\n[Content truncated due to length...]"
}

// truncateNewsContent intelligently truncates news article content
func truncateNewsContent(content string, maxChars int) string {
	articles := strings.Split(content, "\n\n---\n\n")

	var result strings.Builder
	currentLen := 0

	for i, article := range articles {
		articleLen := len(article)
		truncatedMarker := "\n\n---\n\n"

		if i > 0 {
			if currentLen+len(truncatedMarker) > maxChars {
				break
			}
			result.WriteString(truncatedMarker)
			currentLen += len(truncatedMarker)
		}

		if currentLen+articleLen > maxChars {
			remaining := maxChars - currentLen
			if remaining > 100 {
				result.WriteString(article[:remaining])
				result.WriteString("\n\n[Article content truncated...]")
			}
			break
		}

		result.WriteString(article)
		currentLen += articleLen
	}

	return result.String()
}

// findCallerWithToolCallID searches backwards from a tool message to find the assistant
// message that contains the matching tool call ID.
func findCallerWithToolCallID(messages []openai.ChatCompletionMessageParamUnion, toolMsgIndex int, toolCallID string) (int, openai.ChatCompletionMessageParamUnion) {
	// Search backwards from tool message to find assistant with matching tool call
	for i := toolMsgIndex - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.OfAssistant != nil && len(msg.OfAssistant.ToolCalls) > 0 {
			for _, tc := range msg.OfAssistant.ToolCalls {
				if tc.OfFunction != nil && tc.OfFunction.ID == toolCallID {
					return i, msg
				}
			}
		}
	}
	return -1, openai.ChatCompletionMessageParamUnion{}
}

// truncateToolContentMaxTokens truncates content to fit within max tokens.
// Returns empty string if content cannot be trimmed to fit.
func truncateToolContentMaxTokens(content string, maxTokens int) string {
	maxChars := maxTokens * 3
	if maxChars < 300 {
		return "" // Not enough space for meaningful content
	}
	if len(content) <= maxChars {
		return content
	}

	// Check if it's news tool output
	if strings.Contains(content, "## ") && strings.Contains(content, "**Source:**") {
		return truncateNewsContent(content, maxChars)
	}

	return content[:maxChars] + "\n\n[Content truncated due to length...]"
}
