package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) ListMessages(ctx context.Context, req *connect.Request[chatv1.ListMessagesRequest]) (*connect.Response[chatv1.ListMessagesResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	messages, err := s.db.ListMessages(req.Msg.ConversationId, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list messages: %w", err))
	}

	return connect.NewResponse(&chatv1.ListMessagesResponse{
		Messages: messages,
	}), nil
}
