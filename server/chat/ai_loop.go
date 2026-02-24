package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go/v3"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "unblink/server/gen/chat/v1"
	"unblink/server/models"
)

// ResponseSender is the interface for sending events to the client stream.
type ResponseSender interface {
	Send(resp *chatv1.SendMessageResponse) error
}

// MessageSaver is the interface for saving messages and UI blocks.
type MessageSaver interface {
	saveMessage(msg *chatv1.Message) error
	saveUIBlock(block *chatv1.UIBlock) (*chatv1.UIBlock, error)
	getConversationHistory(conversationID string) ([]openai.ChatCompletionMessageParamUnion, error)
	getSystemPrompt(conversationID string) (string, error)
}

// AILoopConfig contains the configuration for running the AI loop.
type AILoopConfig struct {
	OpenAI         *openai.Client
	Tools          *ToolRegistry
	Config         *Config
	ContentTrimmer *models.Trimmer
}

// AILoopResult contains the result of running the AI loop.
type AILoopResult struct {
	// Can be extended if needed
}

// RunAILoop executes the core AI loop with tool calling support.
// It streams responses to the client via the sender and saves messages/blocks via the saver.
func RunAILoop(
	ctx context.Context,
	conversationID string,
	sender ResponseSender,
	saver MessageSaver,
	cfg *AILoopConfig,
) (*AILoopResult, error) {
	if cfg.OpenAI == nil {
		return nil, fmt.Errorf("openai not configured")
	}

	// Fetch History
	history, err := saver.getConversationHistory(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// Get system prompt
	systemPrompt, err := saver.getSystemPrompt(conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system prompt: %w", err)
	}

	// Get available tools from registry
	tools := cfg.Tools.AsOpenAITools()

	// Stream from OpenAI with tool call handling loop (max 5 passes)
	const maxPasses = 5

	for pass := 0; pass < maxPasses; pass++ {
		log.Printf("[ChatService] Pass %d/%d", pass+1, maxPasses)

		// Prepare messages for this API call (trim if needed, but don't modify history)
		messagesToSend := history

		// Prepend system prompt with datetime if configured
		if systemPrompt != "" {
			// Human readable format: "Thursday, January 30, 2026 at 2:30 PM"
			currentDateTime := time.Now().Format("Monday, January 2, 2006 at 3:04 PM")
			systemPromptWithDateTime := fmt.Sprintf("%s\n\nCurrent date/time: %s", systemPrompt, currentDateTime)
			messagesToSend = append([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(systemPromptWithDateTime),
			}, messagesToSend...)
		}

		// Trim content to fit model context
		if cfg.ContentTrimmer != nil {
			originalCount := len(history)
			trimmedHistory, err := cfg.ContentTrimmer.TrimToFit(history)
			if err != nil {
				log.Printf("[ChatService] Warning: content trimming failed: %v", err)
			} else {
				messagesToSend = trimmedHistory
				if len(messagesToSend) < originalCount {
					log.Printf("[ChatService] Trimmed messages from %d to %d to fit context", originalCount, len(messagesToSend))
				}
			}
		}

		// Prepare OpenAI request with trimmed messages
		openAIReq := openai.ChatCompletionNewParams{
			Messages: messagesToSend,
			Tools:    tools,
			Model:    openai.ChatModel(cfg.Config.ChatOpenAIModel),
		}

		// Log messages being sent to OpenAI
		log.Printf("[ChatService] Sending %d messages to OpenAI", len(messagesToSend))

		streamResp := cfg.OpenAI.Chat.Completions.NewStreaming(ctx, openAIReq)
		defer streamResp.Close()

		var fullContent string
		var fullReasoningContent string
		acc := openai.ChatCompletionAccumulator{}
		var toolCalls []openai.FinishedChatCompletionToolCall

		// Assistant block ID and flag
		var assistantUIBlock *chatv1.UIBlock
		assistantBlockSent := false

		// Reasoning block ID, timestamp, and flag
		var reasoningUIBlock *chatv1.UIBlock
		reasoningBlockSent := false

		// Stream loop
		for streamResp.Next() {
			chunk := streamResp.Current()
			acc.AddChunk(chunk)

			// Check for content deltas
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta.Content
				if delta != "" {
					// Create assistant block on first content delta
					if !assistantBlockSent {
						assistantBlockData, _ := json.Marshal(map[string]any{"content": ""})
						assistantUIBlock = &chatv1.UIBlock{
							Id:             generateID(),
							ConversationId: conversationID,
							Role:           "assistant",
							Data:           string(assistantBlockData),
							CreatedAt:      timestamppb.New(time.Now()),
						}
						if err := sender.Send(&chatv1.SendMessageResponse{
							Event: &chatv1.SendMessageResponse_UiBlock{
								UiBlock: assistantUIBlock,
							},
						}); err != nil {
							return nil, fmt.Errorf("stream send error: %w", err)
						}
						assistantBlockSent = true
					}

					fullContent += delta
					// Send Delta event to client
					if err := sender.Send(&chatv1.SendMessageResponse{
						Event: &chatv1.SendMessageResponse_Delta{
							Delta: &chatv1.Delta{
								BlockId: assistantUIBlock.Id,
								Delta:   delta,
							},
						},
					}); err != nil {
						return nil, fmt.Errorf("stream send error: %w", err)
					}
				}

				// Check for reasoning_content or reasoning in the raw JSON
				if len(chunk.Choices[0].Delta.JSON.ExtraFields) > 0 {
					// Try "reasoning_content" first (o1 models), then "reasoning" (other models)
					var reasoningRaw string
					if reasoningField, ok := chunk.Choices[0].Delta.JSON.ExtraFields["reasoning_content"]; ok {
						reasoningRaw = reasoningField.Raw()
					} else if reasoningField, ok := chunk.Choices[0].Delta.JSON.ExtraFields["reasoning"]; ok {
						reasoningRaw = reasoningField.Raw()
					}

					if reasoningRaw != "" {
						// Raw() returns JSON-encoded string like "\"hello\"", so unmarshal it
						var reasoningStr string
						if err := json.Unmarshal([]byte(reasoningRaw), &reasoningStr); err == nil {
							fullReasoningContent += reasoningStr

							// Create reasoning block on first reasoning delta
							if !reasoningBlockSent {
								reasoningBlockData, _ := json.Marshal(map[string]any{"content": ""})
								reasoningUIBlock = &chatv1.UIBlock{
									Id:             generateID(),
									ConversationId: conversationID,
									Role:           "reasoning",
									Data:           string(reasoningBlockData),
									CreatedAt:      timestamppb.New(time.Now()),
								}
								if err := sender.Send(&chatv1.SendMessageResponse{
									Event: &chatv1.SendMessageResponse_UiBlock{
										UiBlock: reasoningUIBlock,
									},
								}); err != nil {
									return nil, fmt.Errorf("stream send error: %w", err)
								}
								reasoningBlockSent = true
							}

							// Send reasoning Delta event to client
							if err := sender.Send(&chatv1.SendMessageResponse{
								Event: &chatv1.SendMessageResponse_Delta{
									Delta: &chatv1.Delta{
										BlockId: reasoningUIBlock.Id,
										Delta:   reasoningStr,
									},
								},
							}); err != nil {
								return nil, fmt.Errorf("stream send error: %w", err)
							}
						}
					}
				}
			}

			// Check for finished tool calls
			if tool, ok := acc.JustFinishedToolCall(); ok {
				log.Printf("[ChatService] Tool call finished: %s (id: %s)", tool.Name, tool.ID)
				toolCalls = append(toolCalls, tool)
			}
		}

		if err := streamResp.Err(); err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}

		// Validate we have choices
		if len(acc.Choices) == 0 {
			return nil, fmt.Errorf("no choices in accumulator response")
		}

		assistantMsgJSON, _ := json.Marshal(acc.Choices[0].Message)
		assistantMsg := &chatv1.Message{
			Id:             generateID(),
			ConversationId: conversationID,
			Body:           string(assistantMsgJSON),
			CreatedAt:      timestamppb.New(time.Now()),
		}
		if err := saver.saveMessage(assistantMsg); err != nil {
			log.Printf("[ChatService] Failed to save assistant message: %v", err)
		}

		// Save and update reasoning block if there's reasoning content
		if fullReasoningContent != "" {
			log.Printf("[ChatService] Full reasoning content: %s", fullReasoningContent)
			sanitizedReasoningContent := sanitizeForPostgres(fullReasoningContent)
			reasoningDataJSON, _ := json.Marshal(map[string]any{
				"content": sanitizedReasoningContent,
			})
			reasoningUIBlock.Data = string(reasoningDataJSON)
			reasoningUIBlock, err = saver.saveUIBlock(reasoningUIBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save reasoning UI block: %v", err)
			} else {
				if err := sender.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: reasoningUIBlock,
					},
				}); err != nil {
					return nil, err
				}
			}
		}

		// Save and update assistant block (if any content was generated)
		if assistantUIBlock != nil {
			sanitizedContent := sanitizeForPostgres(fullContent)
			assistantDataJSON, _ := json.Marshal(map[string]any{
				"content": sanitizedContent,
			})
			assistantUIBlock.Data = string(assistantDataJSON)
			assistantUIBlock, err = saver.saveUIBlock(assistantUIBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save assistant UI block: %v", err)
			} else {
				if err := sender.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: assistantUIBlock,
					},
				}); err != nil {
					return nil, err
				}
			}
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			log.Printf("[ChatService] Completed with no tool calls on pass %d", pass+1)
			return &AILoopResult{}, nil
		}

		// Append assistant message with tool calls to history
		history = append(history, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: func() []openai.ChatCompletionMessageToolCallUnionParam {
					calls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(toolCalls))
					for i, tc := range toolCalls {
						calls[i] = openai.ChatCompletionMessageToolCallUnionParam{
							OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
								ID: tc.ID,
								Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
									Name:      tc.Name,
									Arguments: tc.Arguments,
								},
							},
						}
					}
					return calls
				}(),
			},
		})

		// Execute each tool call and save tool messages
		for i, toolCall := range toolCalls {
			log.Printf("[ChatService] Executing tool %d/%d: %s", i+1, len(toolCalls), toolCall.Name)

			// Use OpenAI's tool call ID as stable ID for replacement
			toolBlockID := toolCall.ID

			// Get the tool to extract display message
			tool, ok := cfg.Tools.Get(toolCall.Name)
			var displayMessage string
			if ok {
				displayMessage = GetDisplayMessage(tool, toolCall.Arguments)
			} else {
				displayMessage = fmt.Sprintf("Running %s", toolCall.Name)
			}

			// Send and save UI block for tool invoked state
			toolInvokedData, _ := json.Marshal(map[string]any{
				"toolName":       toolCall.Name,
				"state":          "invoked",
				"displayMessage": displayMessage,
			})
			toolInvokedBlock := &chatv1.UIBlock{
				Id:             toolBlockID,
				ConversationId: conversationID,
				Role:           "tool",
				Data:           string(toolInvokedData),
				CreatedAt:      timestamppb.New(time.Now()),
			}
			toolInvokedBlock, err = saver.saveUIBlock(toolInvokedBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save tool invoked UI block: %v", err)
			} else {
				if err := sender.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: toolInvokedBlock,
					},
				}); err != nil {
					return nil, err
				}
			}

			// Execute tool and catch any errors during save operations
			var result string
			var saveError error

			// Execute the tool
			result = cfg.Tools.Execute(ctx, toolCall.Name, toolCall.Arguments)

			// Sanitize result for PostgreSQL JSONB compatibility
			result = sanitizeForPostgres(result)

			// Try to save UI block for tool completed state
			toolCompletedData, _ := json.Marshal(map[string]any{
				"toolName":       toolCall.Name,
				"state":          "completed",
				"displayMessage": displayMessage,
				"content":        result,
			})
			toolInvokedBlock.Data = string(toolCompletedData)
			toolCompletedBlock, err := saver.saveUIBlock(toolInvokedBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save tool completed UI block: %v", err)
				saveError = err
			} else {
				if err := sender.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: toolCompletedBlock,
					},
				}); err != nil {
					return nil, err
				}
			}

			// Try to save tool message to database
			toolBodyJSON, _ := json.Marshal(map[string]any{
				"role":         "tool",
				"tool_call_id": toolCall.ID,
				"content":      result,
			})
			toolMsg := &chatv1.Message{
				Id:             generateID(),
				ConversationId: conversationID,
				Body:           string(toolBodyJSON),
				CreatedAt:      timestamppb.New(time.Now()),
			}
			if err := saver.saveMessage(toolMsg); err != nil {
				log.Printf("[ChatService] Failed to save tool message: %v", err)
				saveError = err
			}

			// If any save operation failed, inform the model by appending error to result
			if saveError != nil {
				result = "Error: The system encountered an issue processing this request. Please try a different approach or rephrase your request."

				// Send error UI block to client (no database save, just stream it)
				errorUIBlockData, _ := json.Marshal(map[string]any{
					"toolName":       toolCall.Name,
					"state":          "error",
					"displayMessage": displayMessage,
					"content":        result,
				})
				errorUIBlock := &chatv1.UIBlock{
					Id:             toolBlockID,
					ConversationId: conversationID,
					Role:           "tool",
					Data:           string(errorUIBlockData),
					CreatedAt:      timestamppb.New(time.Now()),
				}
				// Send to client stream (best effort, ignore stream errors here)
				_ = sender.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: errorUIBlock,
					},
				})

				log.Printf("[ChatService] Injected error message into history for tool call %s", toolCall.ID)

				// Try one more time to save the error message (best effort)
				errorMsg := &chatv1.Message{
					Id:             generateID(),
					ConversationId: conversationID,
					Body:           string(toolBodyJSON), // Use same JSON format
					CreatedAt:      timestamppb.New(time.Now()),
				}
				_ = saver.saveMessage(errorMsg) // Ignore error on error message save
			}

			// Always append tool result to history (either actual result or error)
			history = append(history, openai.ToolMessage(result, toolCall.ID))
		}

		// Continue conversation with tool results
		// Note: We don't fetch history again - we've been building it in memory
		// by appending tool messages (either real results or errors) to the history array
		log.Printf("[ChatService] Completed pass %d with %d tool calls, continuing...", pass+1, len(toolCalls))
	}

	// Max passes reached - should not normally get here if model stops calling tools
	log.Printf("[ChatService] Reached max passes (%d)", maxPasses)
	return &AILoopResult{}, nil
}
