package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	"github.com/openai/openai-go/v3"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) SendMessage(ctx context.Context, req *connect.Request[chatv1.SendMessageRequest], stream *connect.ServerStream[chatv1.SendMessageResponse]) error {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	conversationID := req.Msg.ConversationId
	content := sanitizeForPostgres(req.Msg.Content)

	// 1. Save User Message (as JSON body)
	userBody := map[string]any{
		"role":    "user",
		"content": content,
	}
	userBodyJSON, _ := json.Marshal(userBody)
	userMsg := &chatv1.Message{
		Id:             generateID(),
		ConversationId: conversationID,
		Body:           string(userBodyJSON),
		CreatedAt:      timestamppb.New(time.Now()),
	}
	if err := s.saveMessage(userMsg); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save user message: %w", err))
	}

	// Send and save UI block for user message
	userUIBlockData, _ := json.Marshal(map[string]any{
		"content": content,
	})
	userUIBlock, err := s.saveUIBlock(&chatv1.UIBlock{
		Id:             generateID(),
		ConversationId: conversationID,
		Role:           "user",
		Data:           string(userUIBlockData),
		CreatedAt:      timestamppb.New(time.Now()),
	})
	if err != nil {
		log.Printf("[ChatService] Failed to save user UI block: %v", err)
	} else {
		// Send UI block event to client
		if err := stream.Send(&chatv1.SendMessageResponse{
			Event: &chatv1.SendMessageResponse_UiBlock{
				UiBlock: userUIBlock,
			},
		}); err != nil {
			return err
		}
	}

	// 2. Check if we have OpenAI configured
	if s.openai == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("openai not configured"))
	}

	// 3. Fetch History
	history, err := s.getConversationHistory(conversationID, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get history: %w", err))
	}

	// 4. Get available tools from registry
	tools := s.tools.AsOpenAITools()

	// 5. Stream from OpenAI with tool call handling loop (max 5 passes)
	const maxPasses = 5

	for pass := 0; pass < maxPasses; pass++ {
		log.Printf("[ChatService] Pass %d/%d", pass+1, maxPasses)

		// Prepare messages for this API call (trim if needed, but don't modify history)
		messagesToSend := history

		// Trim content to fit model context
		if s.contentTrimmer != nil {
			originalCount := len(history)
			trimmedHistory, err := s.contentTrimmer.TrimToFit(history)
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
			Model:    openai.ChatModel(s.cfg.ChatOpenAIModel),
		}

		// Log messages being sent to OpenAI
		log.Printf("[ChatService] Sending %d messages to OpenAI", len(history))

		streamResp := s.openai.Chat.Completions.NewStreaming(ctx, openAIReq)
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
						if err := stream.Send(&chatv1.SendMessageResponse{
							Event: &chatv1.SendMessageResponse_UiBlock{
								UiBlock: assistantUIBlock,
							},
						}); err != nil {
							return connect.NewError(connect.CodeInternal, fmt.Errorf("stream send error: %w", err))
						}
						assistantBlockSent = true
					}

					fullContent += delta
					// Send Delta event to client
					if err := stream.Send(&chatv1.SendMessageResponse{
						Event: &chatv1.SendMessageResponse_Delta{
							Delta: &chatv1.Delta{
								BlockId: assistantUIBlock.Id,
								Delta:   delta,
							},
						},
					}); err != nil {
						return connect.NewError(connect.CodeInternal, fmt.Errorf("stream send error: %w", err))
					}
				}

				// Check for reasoning_content in the raw JSON
				if len(chunk.Choices[0].Delta.JSON.ExtraFields) > 0 {
					if reasoningField, ok := chunk.Choices[0].Delta.JSON.ExtraFields["reasoning_content"]; ok {
						reasoningRaw := reasoningField.Raw()
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
									if err := stream.Send(&chatv1.SendMessageResponse{
										Event: &chatv1.SendMessageResponse_UiBlock{
											UiBlock: reasoningUIBlock,
										},
									}); err != nil {
										return connect.NewError(connect.CodeInternal, fmt.Errorf("stream send error: %w", err))
									}
									reasoningBlockSent = true
								}

								// Send reasoning Delta event to client
								if err := stream.Send(&chatv1.SendMessageResponse{
									Event: &chatv1.SendMessageResponse_Delta{
										Delta: &chatv1.Delta{
											BlockId: reasoningUIBlock.Id,
											Delta:   reasoningStr,
										},
									},
								}); err != nil {
									return connect.NewError(connect.CodeInternal, fmt.Errorf("stream send error: %w", err))
								}
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
			return connect.NewError(connect.CodeInternal, fmt.Errorf("stream error: %w", err))
		}

		// Validate we have choices
		if len(acc.Choices) == 0 {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("no choices in accumulator response"))
		}

		assistantMsgJSON, _ := json.Marshal(acc.Choices[0].Message)
		assistantMsg := &chatv1.Message{
			Id:             generateID(),
			ConversationId: conversationID,
			Body:           string(assistantMsgJSON),
			CreatedAt:      timestamppb.New(time.Now()),
		}
		if err := s.saveMessage(assistantMsg); err != nil {
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
			reasoningUIBlock, err = s.saveUIBlock(reasoningUIBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save reasoning UI block: %v", err)
			} else {
				if err := stream.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: reasoningUIBlock,
					},
				}); err != nil {
					return err
				}
			}
		}

		// Save and update assistant block
		sanitizedContent := sanitizeForPostgres(fullContent)
		assistantDataJSON, _ := json.Marshal(map[string]any{
			"content": sanitizedContent,
		})
		assistantUIBlock.Data = string(assistantDataJSON)
		assistantUIBlock, err = s.saveUIBlock(assistantUIBlock)
		if err != nil {
			log.Printf("[ChatService] Failed to save assistant UI block: %v", err)
		} else {
			if err := stream.Send(&chatv1.SendMessageResponse{
				Event: &chatv1.SendMessageResponse_UiBlock{
					UiBlock: assistantUIBlock,
				},
			}); err != nil {
				return err
			}
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			log.Printf("[ChatService] Completed with no tool calls on pass %d", pass+1)

			// Generate follow-up topic using fast model (with timeout)
			suggestion := s.generateFollowUpSuggestions(ctx, content, sanitizedContent)
			if suggestion != "" {
				log.Printf("[ChatService] Follow-up suggestion: %s", suggestion)
				// Send to client before returning
				if err := stream.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_FollowUp{
						FollowUp: &chatv1.FollowUp{
							Topic: suggestion,
						},
					},
				}); err != nil {
					log.Printf("[ChatService] Failed to send follow-up: %v", err)
				}
			}

			return nil
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
			tool, ok := s.tools.Get(toolCall.Name)
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
			toolInvokedBlock, err = s.saveUIBlock(toolInvokedBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save tool invoked UI block: %v", err)
			} else {
				if err := stream.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: toolInvokedBlock,
					},
				}); err != nil {
					return err
				}
			}

			// Execute tool and catch any errors during save operations
			var result string
			var saveError error

			// Execute the tool
			result = s.tools.Execute(ctx, toolCall.Name, toolCall.Arguments)

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
			toolCompletedBlock, err := s.saveUIBlock(toolInvokedBlock)
			if err != nil {
				log.Printf("[ChatService] Failed to save tool completed UI block: %v", err)
				saveError = err
			} else {
				if err := stream.Send(&chatv1.SendMessageResponse{
					Event: &chatv1.SendMessageResponse_UiBlock{
						UiBlock: toolCompletedBlock,
					},
				}); err != nil {
					return err
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
			if err := s.saveMessage(toolMsg); err != nil {
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
				_ = stream.Send(&chatv1.SendMessageResponse{
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
				_ = s.saveMessage(errorMsg) // Ignore error on error message save
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
	return nil
}

// saveUIBlock saves or updates a UI block with a specific ID and timestamp.
// The block's Data field should already be JSON-encoded.
func (s *Service) saveUIBlock(block *chatv1.UIBlock) (*chatv1.UIBlock, error) {
	finalTimestamp := block.CreatedAt
	if finalTimestamp == nil {
		finalTimestamp = timestamppb.Now()
	}

	err := s.db.StoreUIBlock(block.Id, block.ConversationId, block.Role, block.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to update UI block: %w", err)
	}

	return &chatv1.UIBlock{
		Id:             block.Id,
		ConversationId: block.ConversationId,
		Role:           block.Role,
		Data:           block.Data,
		CreatedAt:      finalTimestamp,
	}, nil
}

func (s *Service) saveMessage(msg *chatv1.Message) error {
	return s.db.StoreMessage(msg.Id, msg.ConversationId, msg.Body)
}

// generateFollowUpSuggestions generates an interesting follow-up topic
// using the last user and assistant message pair.
func (s *Service) generateFollowUpSuggestions(ctx context.Context, userContent, assistantContent string) string {
	// Skip if fast client not configured
	if s.fastOpenai == nil {
		return ""
	}

	// Trim inputs to reasonable length for follow-up generation
	// We only need the gist, not every detail
	const maxContentLen = 2000
	if len(userContent) > maxContentLen {
		userContent = userContent[:maxContentLen] + "..."
	}
	if len(assistantContent) > maxContentLen {
		assistantContent = assistantContent[:maxContentLen] + "..."
	}

	// Call fast model
	resp, err := s.fastOpenai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You output concise follow-up topics as JSON with a 'topic' field."),
			openai.UserMessage(fmt.Sprintf(
				`Based on this conversation, suggest ONE interesting follow-up topic that the human can ask AI.

Return JSON like: {"topic": "Your topic here"}

User: %s

Assistant: %s`,
				userContent, assistantContent,
			)),
		},
		Model: openai.ChatModel(s.cfg.FastOpenAIModel),
	})
	if err != nil {
		log.Printf("[ChatService] Fast model follow-up generation failed: %v", err)
		return ""
	}

	// Parse JSON response
	if len(resp.Choices) == 0 {
		return ""
	}

	var result struct {
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		log.Printf("[ChatService] Failed to parse follow-up suggestion: %v", err)
		return ""
	}

	log.Printf("[ChatService] Generated follow-up: %s", result.Topic)
	return result.Topic
}

func (s *Service) getConversationHistory(conversationID, userID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	// First get system prompt with ownership verification
	systemPrompt, err := s.db.GetSystemPrompt(conversationID, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var messages []openai.ChatCompletionMessageParamUnion
	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}

	// Get messages with ownership verification
	bodies, err := s.db.GetMessagesBody(conversationID, userID)
	if err != nil {
		return nil, err
	}

	for _, body := range bodies {
		// Use flexible wrapper since ChatCompletionMessage has Role hardcoded as Assistant
		var wrapper struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			// Tool calls
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
			ToolCallID string `json:"tool_call_id,omitempty"`
		}
		if err := json.Unmarshal([]byte(body), &wrapper); err != nil {
			log.Printf("[ChatService] Failed to unmarshal message body: %v", err)
			continue
		}

		// Convert to correct param type based on role
		switch wrapper.Role {
		case "system":
			messages = append(messages, openai.SystemMessage(wrapper.Content))
		case "user":
			messages = append(messages, openai.UserMessage(wrapper.Content))
		case "assistant":
			if len(wrapper.ToolCalls) > 0 {
				// Assistant with tool calls
				toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(wrapper.ToolCalls))
				for i, tc := range wrapper.ToolCalls {
					toolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						},
					}
				}
				// Create assistant message param with tool calls, no content
				assistant := openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				messages = append(messages, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
			} else {
				messages = append(messages, openai.AssistantMessage(wrapper.Content))
			}
		case "tool":
			messages = append(messages, openai.ToolMessage(wrapper.Content, wrapper.ToolCallID))
		default:
			log.Printf("[ChatService] Unknown message role: %s", wrapper.Role)
		}
	}
	return messages, nil
}
