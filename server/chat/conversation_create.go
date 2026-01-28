package chat

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"unblink/server/internal/ctxutil"
	chatv1 "unblink/server/gen/chat/v1"
)

// GetUserIDFromContext extracts the user ID from the context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	return ctxutil.GetUserIDFromContext(ctx)
}

func (s *Service) CreateConversation(ctx context.Context, req *connect.Request[chatv1.CreateConversationRequest]) (*connect.Response[chatv1.CreateConversationResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	id := generateID()
	now := time.Now()

	title := req.Msg.Title
	if title == "" {
		title = "New Chat"
	}

	err := s.db.CreateConversation(id, userID, title, req.Msg.SystemPrompt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create conversation: %w", err))
	}

	return connect.NewResponse(&chatv1.CreateConversationResponse{
		Conversation: &chatv1.Conversation{
			Id:           id,
			Title:        title,
			SystemPrompt: req.Msg.SystemPrompt,
			CreatedAt:    timestamppb.New(now),
			UpdatedAt:    timestamppb.New(now),
		},
	}), nil
}
