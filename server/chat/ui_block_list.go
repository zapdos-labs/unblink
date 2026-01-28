package chat

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) ListUIBlocks(ctx context.Context, req *connect.Request[chatv1.ListUIBlocksRequest]) (*connect.Response[chatv1.ListUIBlocksResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	blocks, err := s.db.ListUIBlocks(req.Msg.ConversationId, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list UI blocks: %w", err))
	}

	return connect.NewResponse(&chatv1.ListUIBlocksResponse{
		UiBlocks: blocks,
	}), nil
}
