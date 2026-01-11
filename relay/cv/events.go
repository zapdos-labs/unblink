package cv

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// FrameEvent represents a single frame event
type FrameEvent struct {
	ServiceID string `json:"service_id"`
	FrameUUID string `json:"frame_uuid"`
}

// FrameBatchEvent represents a batch of frames event
type FrameBatchEvent struct {
	ServiceID string                 `json:"service_id"`
	Frames    []string               `json:"frames"` // Frame UUIDs
	Metadata  map[string]interface{} `json:"metadata"`
}

// WorkerEvent represents an event emitted by a worker
type WorkerEvent struct {
	EventID   string                 `json:"event_id"`
	WorkerID  string                 `json:"worker_id"`
	CreatedAt time.Time              `json:"created_at"`
	Data      map[string]interface{} `json:"data"` // Flexible event data (summary, metrics, alerts, etc.)
}

// CVEventBus manages event broadcasting and worker event collection
type CVEventBus struct {
	// Event listeners
	frameListeners      []func(*FrameEvent)
	frameBatchListeners []func(*FrameBatchEvent)
	workerEventHandlers []func(*WorkerEvent)
	mu                  sync.RWMutex

	// Worker event storage (in-memory for now)
	workerEvents []*WorkerEvent
	eventsMu     sync.RWMutex
}

// NewCVEventBus creates a new event bus
func NewCVEventBus() *CVEventBus {
	return &CVEventBus{
		frameListeners:      make([]func(*FrameEvent), 0),
		frameBatchListeners: make([]func(*FrameBatchEvent), 0),
		workerEventHandlers: make([]func(*WorkerEvent), 0),
		workerEvents:        make([]*WorkerEvent, 0),
	}
}

// OnFrameEvent registers a listener for frame events
func (bus *CVEventBus) OnFrameEvent(handler func(*FrameEvent)) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.frameListeners = append(bus.frameListeners, handler)
}

// OnFrameBatchEvent registers a listener for frame batch events
func (bus *CVEventBus) OnFrameBatchEvent(handler func(*FrameBatchEvent)) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.frameBatchListeners = append(bus.frameBatchListeners, handler)
}

// OnWorkerEvent registers a handler for worker events
func (bus *CVEventBus) OnWorkerEvent(handler func(*WorkerEvent)) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.workerEventHandlers = append(bus.workerEventHandlers, handler)
}

// EmitFrameEvent broadcasts a frame event
func (bus *CVEventBus) EmitFrameEvent(event *FrameEvent) {
	bus.mu.RLock()
	listeners := bus.frameListeners
	bus.mu.RUnlock()

	for _, listener := range listeners {
		go listener(event)
	}

	log.Printf("[CVEventBus] Emitted frame event (service=%s, frame=%s)",
		event.ServiceID, event.FrameUUID)
}

// EmitFrameBatchEvent broadcasts a frame batch event
func (bus *CVEventBus) EmitFrameBatchEvent(event *FrameBatchEvent) {
	bus.mu.RLock()
	listeners := bus.frameBatchListeners
	bus.mu.RUnlock()

	for _, listener := range listeners {
		go listener(event)
	}

	log.Printf("[CVEventBus] Emitted frame batch event (service=%s, frames=%d)",
		event.ServiceID, len(event.Frames))
}

// PublishWorkerEvent stores and processes a worker event
func (bus *CVEventBus) PublishWorkerEvent(event *WorkerEvent) {
	// Store event
	bus.eventsMu.Lock()
	bus.workerEvents = append(bus.workerEvents, event)
	bus.eventsMu.Unlock()

	// Notify handlers
	bus.mu.RLock()
	handlers := bus.workerEventHandlers
	bus.mu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}

	// Log event data
	if summary, ok := event.Data["summary"].(string); ok {
		log.Printf("[CVEventBus] Worker %s: %s", event.WorkerID, summary)
	} else {
		data, _ := json.Marshal(event.Data)
		log.Printf("[CVEventBus] Worker %s event: %s", event.WorkerID, string(data))
	}
}

// GetWorkerEvents retrieves recent worker events (limited to last 1000)
func (bus *CVEventBus) GetWorkerEvents(limit int) []*WorkerEvent {
	bus.eventsMu.RLock()
	defer bus.eventsMu.RUnlock()

	if limit <= 0 || limit > len(bus.workerEvents) {
		limit = len(bus.workerEvents)
	}

	// Return most recent events
	start := len(bus.workerEvents) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*WorkerEvent, limit)
	copy(events, bus.workerEvents[start:])
	return events
}
