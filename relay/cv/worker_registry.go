package cv

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

// CVWorker represents a connected worker
type CVWorker struct {
	ID           string
	Conn         *websocket.Conn
	RegisteredAt time.Time
	LastSeen     time.Time
	sendChan     chan []byte
	closeChan    chan struct{}
	closeOnce    sync.Once
}

// Close safely closes the worker's close channel
func (w *CVWorker) Close() {
	w.closeOnce.Do(func() {
		close(w.closeChan)
	})
}

// CVWorkerRegistry manages worker connections and event distribution
type CVWorkerRegistry struct {
	workers        map[string]*CVWorker // workerID → worker
	workerKeys     map[string]string    // key → workerID (for authentication)
	mu             sync.RWMutex
	eventBus       *CVEventBus
	storageManager *StorageManager
	upgrader       websocket.Upgrader
}

// WebSocket message types
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type RegisterMessage struct {
	// No fields needed - relay generates worker_id
}

type RegisteredMessage struct {
	WorkerID string `json:"worker_id"`
}

type HeartbeatMessage struct {
	// No fields needed - relay tracks workers by connection
}

// NewCVWorkerRegistry creates a new worker registry
func NewCVWorkerRegistry(eventBus *CVEventBus, storageManager *StorageManager) *CVWorkerRegistry {
	registry := &CVWorkerRegistry{
		workers:        make(map[string]*CVWorker),
		workerKeys:     make(map[string]string),
		eventBus:       eventBus,
		storageManager: storageManager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}

	// Register event listeners to broadcast to workers
	eventBus.OnFrameEvent(func(event *FrameEvent) {
		registry.BroadcastFrameEvent(event, time.Now())
	})
	eventBus.OnFrameBatchEvent(func(event *FrameBatchEvent) {
		registry.BroadcastFrameBatchEvent(event, time.Now())
	})

	return registry
}

// generateWorkerKey generates a unique cryptographic key for worker authentication
func generateWorkerKey() string {
	keyBytes := make([]byte, 32) // 256-bit key
	rand.Read(keyBytes)
	return hex.EncodeToString(keyBytes)
}

// GetWorkerIDByKey retrieves the worker ID associated with a key
func (r *CVWorkerRegistry) GetWorkerIDByKey(key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	workerID, exists := r.workerKeys[key]
	return workerID, exists
}

// registerWorkerKey registers a worker's authentication key
func (r *CVWorkerRegistry) registerWorkerKey(key string, workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workerKeys[key] = workerID
}

// SetStorageManager sets the storage manager (used to resolve circular dependency)
func (r *CVWorkerRegistry) SetStorageManager(sm *StorageManager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.storageManager = sm
}

// HandleWebSocket handles WebSocket connection requests
func (r *CVWorkerRegistry) HandleWebSocket(w http.ResponseWriter, req *http.Request) {
	// Upgrade connection
	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("[CVWorkerRegistry] Failed to upgrade connection: %v", err)
		return
	}

	// Handle worker connection
	go r.handleWorkerConnection(conn)
}

// handleWorkerConnection handles a single worker WebSocket connection
func (r *CVWorkerRegistry) handleWorkerConnection(conn *websocket.Conn) {
	defer conn.Close()

	// Wait for registration message
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		log.Printf("[CVWorkerRegistry] Failed to read registration message: %v", err)
		return
	}

	if msg.Type != "register" {
		log.Printf("[CVWorkerRegistry] Expected register message, got: %s", msg.Type)
		return
	}

	// Generate worker ID relay-side
	workerID := uuid.New().String()

	// Create worker
	worker := &CVWorker{
		ID:           workerID,
		Conn:         conn,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		sendChan:     make(chan []byte, 100),
		closeChan:    make(chan struct{}),
	}

	// Generate unique key for this worker
	workerKey := generateWorkerKey()

	// Register worker and key
	r.RegisterWorker(worker)
	r.registerWorkerKey(workerKey, workerID)
	defer r.RemoveWorker(workerID)

	// Send registration confirmation with key
	regConfirm := map[string]interface{}{
		"type": "registered",
		"data": map[string]string{
			"worker_id": workerID,
			"key":       workerKey,
		},
	}
	if data, err := json.Marshal(regConfirm); err == nil {
		worker.sendChan <- data
	}

	log.Printf("[CVWorkerRegistry] Worker %s connected", workerID)

	// Start send and receive goroutines
	go r.sendLoop(worker)
	r.receiveLoop(worker)
}

// sendLoop handles sending messages to worker
func (r *CVWorkerRegistry) sendLoop(worker *CVWorker) {
	defer worker.Conn.Close()

	for {
		select {
		case <-worker.closeChan:
			return
		case data := <-worker.sendChan:
			if err := worker.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[CVWorkerRegistry] Failed to send to worker %s: %v", worker.ID, err)
				return
			}
		}
	}
}

// receiveLoop handles receiving messages from worker
func (r *CVWorkerRegistry) receiveLoop(worker *CVWorker) {
	defer worker.Close()

	for {
		var msg WSMessage
		if err := worker.Conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[CVWorkerRegistry] Worker %s connection error: %v", worker.ID, err)
			}
			return
		}

		// Handle message
		switch msg.Type {
		case "heartbeat":
			r.mu.Lock()
			worker.LastSeen = time.Now()
			r.mu.Unlock()

		case "event":
			// Parse event data
			var eventData map[string]interface{}
			if err := json.Unmarshal(msg.Data, &eventData); err != nil {
				log.Printf("[CVWorkerRegistry] Failed to parse event from worker %s: %v", worker.ID, err)
				continue
			}

			// Create worker event
			workerEvent := &WorkerEvent{
				EventID:   fmt.Sprintf("%s-%d", worker.ID, time.Now().UnixNano()),
				WorkerID:  worker.ID,
				CreatedAt: time.Now(),
				Data:      eventData,
			}

			// Publish to event bus
			r.eventBus.PublishWorkerEvent(workerEvent)

		default:
			log.Printf("[CVWorkerRegistry] Unknown message type from worker %s: %s", worker.ID, msg.Type)
		}
	}
}

// RegisterWorker adds a worker to the registry
func (r *CVWorkerRegistry) RegisterWorker(worker *CVWorker) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove old connection if exists
	if old, exists := r.workers[worker.ID]; exists {
		old.Close()
	}

	r.workers[worker.ID] = worker
	log.Printf("[CVWorkerRegistry] Registered worker %s (total=%d)", worker.ID, len(r.workers))
}

// RemoveWorker removes a worker from the registry
func (r *CVWorkerRegistry) RemoveWorker(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if worker, exists := r.workers[workerID]; exists {
		worker.Close()
		delete(r.workers, workerID)

		// Remove all keys associated with this worker
		for key, wID := range r.workerKeys {
			if wID == workerID {
				delete(r.workerKeys, key)
			}
		}

		log.Printf("[CVWorkerRegistry] Removed worker %s (total=%d)", workerID, len(r.workers))
	}
}

// BroadcastFrameEvent broadcasts a frame event to all connected workers
func (r *CVWorkerRegistry) BroadcastFrameEvent(event *FrameEvent, timestamp time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.workers) == 0 {
		return
	}

	msg := map[string]interface{}{
		"type":       "frame",
		"id":         uuid.New().String(),
		"created_at": timestamp.UTC().Format(time.RFC3339),
		"data":       event,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CVWorkerRegistry] Failed to marshal frame event: %v", err)
		return
	}

	// Send to all workers
	for _, worker := range r.workers {
		select {
		case worker.sendChan <- data:
		default:
			log.Printf("[CVWorkerRegistry] Worker %s send channel full, skipping frame event", worker.ID)
		}
	}
}

// BroadcastFrameBatchEvent broadcasts a frame batch event to all connected workers
func (r *CVWorkerRegistry) BroadcastFrameBatchEvent(event *FrameBatchEvent, timestamp time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.workers) == 0 {
		return
	}

	msg := map[string]interface{}{
		"type":       "frame_batch",
		"id":         uuid.New().String(),
		"created_at": timestamp.UTC().Format(time.RFC3339),
		"data":       event,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CVWorkerRegistry] Failed to marshal frame batch event: %v", err)
		return
	}

	// Send to all workers
	for _, worker := range r.workers {
		select {
		case worker.sendChan <- data:
		default:
			log.Printf("[CVWorkerRegistry] Worker %s send channel full, skipping frame batch event", worker.ID)
		}
	}
}
