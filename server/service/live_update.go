package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"connectrpc.com/connect"

	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"
	"github.com/zapdos-labs/unblink/server/gen/service/v1/servicev1connect"
	"github.com/zapdos-labs/unblink/server/internal/ctxutil"
)

type LiveUpdateDatabase interface {
	CheckNodeAccess(nodeID, userID string) (bool, error)
}

type LiveUpdateService struct {
	db          LiveUpdateDatabase
	registry    *ServiceRegistry
	broadcaster *LiveUpdateBroadcaster
}

func NewLiveUpdateService(db LiveUpdateDatabase, registry *ServiceRegistry) *LiveUpdateService {
	return &LiveUpdateService{
		db:          db,
		registry:    registry,
		broadcaster: NewLiveUpdateBroadcaster(),
	}
}

func (s *LiveUpdateService) BroadcastNodeStatus(nodeID string, online bool) {
	s.broadcaster.BroadcastNodeStatus(nodeID, online)
}

func (s *LiveUpdateService) StreamLiveUpdates(ctx context.Context, req *connect.Request[servicev1.StreamLiveUpdatesRequest], stream *connect.ServerStream[servicev1.StreamLiveUpdatesResponse]) error {
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	for _, nodeID := range req.Msg.NodeIds {
		if err := ctxutil.CheckNodeAccessWithContext(ctx, s.db, nodeID); err != nil {
			return err
		}
	}

	sub, updates := s.broadcaster.Subscribe(userID, req.Msg.NodeIds)
	defer s.broadcaster.Unsubscribe(sub)

	log.Printf("[LiveUpdateService] Starting stream: user=%s nodeCount=%d", userID, len(req.Msg.NodeIds))

	for _, nodeID := range req.Msg.NodeIds {
		if err := stream.Send(&servicev1.StreamLiveUpdatesResponse{
			Payload: &servicev1.StreamLiveUpdatesResponse_NodeStatusChanged{
				NodeStatusChanged: &servicev1.NodeStatusChanged{
					NodeId: nodeID,
					Online: s.registry.IsNodeOnline(nodeID),
				},
			},
		}); err != nil {
			return err
		}
	}

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[LiveUpdateService] Stream context done: user=%s", userID)
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(update); err != nil {
				return err
			}
		case <-heartbeat.C:
			if err := stream.Send(&servicev1.StreamLiveUpdatesResponse{
				Payload: &servicev1.StreamLiveUpdatesResponse_Heartbeat{
					Heartbeat: fmt.Sprintf("ping:%d", time.Now().Unix()),
				},
			}); err != nil {
				return err
			}
		}
	}
}

var _ servicev1connect.LiveUpdateServiceHandler = (*LiveUpdateService)(nil)
