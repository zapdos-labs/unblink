package chat

import (
	"context"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) ListMessages(ctx context.Context, req *connect.Request[chatv1.ListMessagesRequest]) (*connect.Response[chatv1.ListMessagesResponse], error) {
	// Verify ownership first
	if err := s.verifyConversationOwnership(ctx, req.Msg.ConversationId); err != nil {
		return nil, err
	}

	messages, err := s.db.ListMessages(req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&chatv1.ListMessagesResponse{
		Messages: messages,
	}), nil
}
