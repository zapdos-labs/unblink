package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) DeleteConversation(ctx context.Context, req *connect.Request[chatv1.DeleteConversationRequest]) (*connect.Response[chatv1.DeleteConversationResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	err := s.db.DeleteConversation(req.Msg.ConversationId, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete conversation: %w", err))
	}

	return connect.NewResponse(&chatv1.DeleteConversationResponse{
		Success: true,
	}), nil
}
