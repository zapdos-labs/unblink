package service

import (
	"log"
	"sync"

	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"
)

type LiveUpdateSubscription struct {
	UserID  string
	NodeIDs map[string]struct{}
	Stream  chan *servicev1.StreamLiveUpdatesResponse
}

type LiveUpdateBroadcaster struct {
	mu            sync.RWMutex
	subscriptions map[string]map[*LiveUpdateSubscription]struct{}
}

func NewLiveUpdateBroadcaster() *LiveUpdateBroadcaster {
	return &LiveUpdateBroadcaster{
		subscriptions: make(map[string]map[*LiveUpdateSubscription]struct{}),
	}
}

func (b *LiveUpdateBroadcaster) Subscribe(userID string, nodeIDs []string) (*LiveUpdateSubscription, <-chan *servicev1.StreamLiveUpdatesResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeFilter := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodeFilter[nodeID] = struct{}{}
	}

	sub := &LiveUpdateSubscription{
		UserID:  userID,
		NodeIDs: nodeFilter,
		Stream:  make(chan *servicev1.StreamLiveUpdatesResponse, 32),
	}

	if b.subscriptions[userID] == nil {
		b.subscriptions[userID] = make(map[*LiveUpdateSubscription]struct{})
	}
	b.subscriptions[userID][sub] = struct{}{}

	log.Printf("[LiveUpdateBroadcaster] New subscription: user=%s nodeCount=%d", userID, len(nodeFilter))

	return sub, sub.Stream
}

func (b *LiveUpdateBroadcaster) Unsubscribe(sub *LiveUpdateSubscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	userSubs := b.subscriptions[sub.UserID]
	if userSubs == nil {
		return
	}

	if _, exists := userSubs[sub]; !exists {
		return
	}

	delete(userSubs, sub)
	if len(userSubs) == 0 {
		delete(b.subscriptions, sub.UserID)
	}

	close(sub.Stream)
	log.Printf("[LiveUpdateBroadcaster] Subscription removed: user=%s", sub.UserID)
}

func (b *LiveUpdateBroadcaster) BroadcastNodeStatus(nodeID string, online bool) {
	update := &servicev1.StreamLiveUpdatesResponse{
		Payload: &servicev1.StreamLiveUpdatesResponse_NodeStatusChanged{
			NodeStatusChanged: &servicev1.NodeStatusChanged{
				NodeId: nodeID,
				Online: online,
			},
		},
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for userID, userSubs := range b.subscriptions {
		for sub := range userSubs {
			if len(sub.NodeIDs) > 0 {
				if _, ok := sub.NodeIDs[nodeID]; !ok {
					continue
				}
			}

			select {
			case sub.Stream <- update:
			default:
				log.Printf("[LiveUpdateBroadcaster] Subscription channel full: user=%s node=%s", userID, nodeID)
			}
		}
	}
}
