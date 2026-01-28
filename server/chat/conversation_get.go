package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) GetConversation(ctx context.Context, req *connect.Request[chatv1.GetConversationRequest]) (*connect.Response[chatv1.GetConversationResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// GetConversation now filters by userID, so if not found it means either doesn't exist or user doesn't own it
	conversation, err := s.db.GetConversation(req.Msg.ConversationId, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get conversation: %w", err))
	}

	if conversation == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation not found"))
	}

	return connect.NewResponse(&chatv1.GetConversationResponse{
		Conversation: conversation,
	}), nil
}
