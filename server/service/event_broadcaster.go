package service

import (
	"context"
	"log"
	"sync"

	servicev1 "unblink/server/gen/service/v1"
)

// EventSubscription represents a client's subscription to events
type EventSubscription struct {
	NodeID     string
	ServiceID  string
	Stream     chan *servicev1.Event
	CancelFunc context.CancelFunc
}

// EventBroadcaster manages event broadcasting to connected clients
type EventBroadcaster struct {
	mu            sync.RWMutex
	subscriptions map[string][]*EventSubscription // nodeID -> subscriptions
	allSubs       map[*EventSubscription]string    // reverse lookup: sub -> nodeID
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		subscriptions: make(map[string][]*EventSubscription),
		allSubs:       make(map[*EventSubscription]string),
	}
}

// Subscribe adds a new subscription for a node
// Returns a read-only channel of events and a cancel function
func (b *EventBroadcaster) Subscribe(ctx context.Context, nodeID, serviceID string) (<-chan *servicev1.Event, context.CancelFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	eventChan := make(chan *servicev1.Event, 100) // Buffered channel

	sub := &EventSubscription{
		NodeID:     nodeID,
		ServiceID:  serviceID,
		Stream:     eventChan,
		CancelFunc: cancel,
	}

	b.subscriptions[nodeID] = append(b.subscriptions[nodeID], sub)
	b.allSubs[sub] = nodeID

	log.Printf("[EventBroadcaster] New subscription: node=%s, service=%s", nodeID, serviceID)

	return eventChan, cancel
}

// Unsubscribe removes a subscription
func (b *EventBroadcaster) Unsubscribe(sub *EventSubscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	nodeID := b.allSubs[sub]
	if nodeID == "" {
		return
	}

	subs := b.subscriptions[nodeID]
	for i, s := range subs {
		if s == sub {
			b.subscriptions[nodeID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	delete(b.allSubs, sub)
	close(sub.Stream)

	log.Printf("[EventBroadcaster] Subscription removed: node=%s", nodeID)
}

// Broadcast sends an event to all matching subscriptions
func (b *EventBroadcaster) Broadcast(event *servicev1.Event, nodeID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscriptions[nodeID]
	if len(subs) == 0 {
		return
	}

	sentCount := 0
	for _, sub := range subs {
		// Filter by service_id if specified
		if sub.ServiceID != "" && sub.ServiceID != event.ServiceId {
			continue
		}

		// Non-blocking send
		select {
		case sub.Stream <- event:
			sentCount++
		default:
			// Channel full, log warning
			log.Printf("[EventBroadcaster] Subscription channel full for node=%s", nodeID)
		}
	}

	if sentCount > 0 {
		log.Printf("[EventBroadcaster] Broadcast event %s to %d subscribers (node=%s)",
			event.Id, sentCount, nodeID)
	}
}

// GetSubscriptionCount returns the number of active subscriptions for a node
func (b *EventBroadcaster) GetSubscriptionCount(nodeID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscriptions[nodeID])
}
