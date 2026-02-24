package chat

import (
	"context"

	"connectrpc.com/connect"

	chatv1 "unblink/server/gen/chat/v1"
)

func (s *Service) ListUIBlocks(ctx context.Context, req *connect.Request[chatv1.ListUIBlocksRequest]) (*connect.Response[chatv1.ListUIBlocksResponse], error) {
	// Verify ownership first
	if err := s.verifyConversationOwnership(ctx, req.Msg.ConversationId); err != nil {
		return nil, err
	}

	blocks, err := s.db.ListUIBlocks(req.Msg.ConversationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&chatv1.ListUIBlocksResponse{
		UiBlocks: blocks,
	}), nil
}
