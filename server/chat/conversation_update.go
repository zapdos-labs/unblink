package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) UpdateConversation(ctx context.Context, req *connect.Request[chatv1.UpdateConversationRequest]) (*connect.Response[chatv1.UpdateConversationResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	var title, systemPrompt string
	if req.Msg.Title != nil {
		title = *req.Msg.Title
	}
	if req.Msg.SystemPrompt != nil {
		systemPrompt = *req.Msg.SystemPrompt
	}

	err := s.db.UpdateConversation(req.Msg.ConversationId, userID, title, systemPrompt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update conversation: %w", err))
	}

	// Fetch the updated conversation
	conversation, err := s.db.GetConversation(req.Msg.ConversationId, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get updated conversation: %w", err))
	}

	return connect.NewResponse(&chatv1.UpdateConversationResponse{
		Conversation: conversation,
	}), nil
}
