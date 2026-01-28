package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) ListConversations(ctx context.Context, req *connect.Request[chatv1.ListConversationsRequest]) (*connect.Response[chatv1.ListConversationsResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	conversations, err := s.db.ListConversations(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list conversations: %w", err))
	}

	return connect.NewResponse(&chatv1.ListConversationsResponse{
		Conversations: conversations,
	}), nil
}
