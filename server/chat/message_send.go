package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"
	"github.com/openai/openai-go/v3"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) SendMessage(ctx context.Context, req *connect.Request[chatv1.SendMessageRequest], stream *connect.ServerStream[chatv1.SendMessageResponse]) error {
	conversationID := req.Msg.ConversationId

	// Verify ownership first
	if err := s.verifyConversationOwnership(ctx, conversationID); err != nil {
		return err
	}

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

	// 2. Run the AI loop
	aiConfig := &AILoopConfig{
		OpenAI:         s.openai,
		Tools:          s.tools,
		Config:         s.cfg,
		ContentTrimmer: s.contentTrimmer,
	}
	_, err = RunAILoop(ctx, conversationID, stream, s, aiConfig)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

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

func (s *Service) getConversationHistory(conversationID string) ([]openai.ChatCompletionMessageParamUnion, error) {
	// Note: Authorization is already done at the handler level via verifyConversationOwnership

	var messages []openai.ChatCompletionMessageParamUnion

	// Get message bodies
	bodies, err := s.db.GetMessagesBody(conversationID)
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

func (s *Service) getSystemPrompt(conversationID string) (string, error) {
	return s.db.GetSystemPrompt(conversationID)
}
