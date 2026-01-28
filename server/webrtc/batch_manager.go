package webrtc

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// rollingContext holds the rolling state for a service
type rollingContext struct {
	lastFrame         *Frame // Last frame from previous batch
	previousResponse  string // Last VLM raw response (JSON)
	lastBatchSentTime int64  // Unix nano timestamp of last batch sent (for ordering)
}

// BatchManager accumulates frames and sends them in batches to vLLM
// It maintains rolling context per service, asking the model to describe only what's new
type BatchManager struct {
	client          *FrameClient
	batchSize       int
	frameBatches    map[string][]*Frame        // serviceID -> pending frames
	rollingContexts map[string]*rollingContext // serviceID -> rolling state
	baseInstruction string                     // Base instruction
	storage         *Storage                   // Storage for saving annotated frames
	mu              sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int, storage *Storage) *BatchManager {
	return &BatchManager{
		client:           client,
		batchSize:        batchSize,
		frameBatches:     make(map[string][]*Frame),
		rollingContexts:  make(map[string]*rollingContext),
		baseInstruction:  "Analyze these video frames for motion, action, emotion, facial expressions, and subtle details. Detect ALL objects and return bounding boxes in NORMALIZED 1000 COORDINATES (0=top/left, 1000=bottom/right).",
		storage:          storage,
	}
}

// AddFrame adds a frame to the batch and sends if batch is full
// Frame should already be preprocessed (resized + timestamp burnt in)
func (m *BatchManager) AddFrame(frame *Frame) {
	m.mu.Lock()

	serviceID := frame.ServiceID

	// Initialize batch if needed
	if m.frameBatches[serviceID] == nil {
		m.frameBatches[serviceID] = make([]*Frame, 0, m.batchSize)
	}

	// Add frame to batch
	m.frameBatches[serviceID] = append(m.frameBatches[serviceID], frame)

	// Check if batch is ready to send
	batch := m.frameBatches[serviceID]
	shouldSend := len(batch) >= m.batchSize

	if shouldSend {
		// Copy batch and clear
		framesToSend := make([]*Frame, len(batch))
		copy(framesToSend, batch)
		m.frameBatches[serviceID] = nil

		// Get or create context
		rollCtx := m.rollingContexts[serviceID]
		if rollCtx == nil {
			rollCtx = &rollingContext{}
			m.rollingContexts[serviceID] = rollCtx
		}

		// Store last frame for next batch and capture send timestamp
		newLastFrame := framesToSend[len(framesToSend)-1]
		batchSentTime := time.Now().UnixNano()

		// Snapshot context data for async goroutine (copy values before unlocking)
		prevResponse := rollCtx.previousResponse
		prevLastFrame := rollCtx.lastFrame

		// Update context
		rollCtx.lastFrame = newLastFrame
		rollCtx.lastBatchSentTime = batchSentTime

		m.mu.Unlock()

		// Send batch asynchronously to avoid blocking
		go m.sendBatch(serviceID, framesToSend, prevResponse, prevLastFrame, batchSentTime)
	} else {
		m.mu.Unlock()
	}
}

// sendBatch sends a batch of frames to vLLM with rolling context
func (m *BatchManager) sendBatch(serviceID string, frames []*Frame, previousResponse string, lastFrame *Frame, batchSentTime int64) {
	// Build instruction based on whether we have previous context
	// Note: JSON schema is handled by response_format in FrameClient
	instruction := m.baseInstruction

	if previousResponse != "" {
		instruction = strings.Join([]string{
			m.baseInstruction,
			"",
			"PREVIOUS ANALYSIS (use for object tracking):",
			previousResponse,
			"",
			"IMPORTANT: Use NORMALIZED 1000 COORDINATES for all bounding boxes.",
			"- Coordinates range from 0 to 1000",
			"- 0 = top/left edge, 1000 = bottom/right edge",
			"- For example: [250, 300, 750, 800] means 25%-75% horizontal and 30%-80% vertical",
			"",
			"Detect ALL objects in the current frame. For objects that match previous objects (same type, similar location), use their existing ID.",
			"For new objects, assign new IDs (continue numbering from previous).",
			"Focus on motion, action, emotion, facial expressions, and subtle details.",
			"In the description, reference objects by their IDs in brackets, e.g., 'The person [3] is driving a red car [4]'.",
		}, "\n")
	}

	// Prepend last frame for visual continuity
	framesToSend := frames
	if lastFrame != nil {
		framesToSend = append([]*Frame{lastFrame}, frames...)
		log.Printf("[BatchManager] Sending batch for service %s: %d frames (+1 continuity frame)", serviceID, len(frames))
	} else {
		log.Printf("[BatchManager] Sending batch for service %s: %d frames", serviceID, len(frames))
	}

	ctx := context.Background()
	response, err := m.client.SendFrameBatchWithInstruction(ctx, framesToSend, instruction)
	if err != nil {
		log.Printf("[BatchManager] Failed to send batch for service %s: %v", serviceID, err)
		return
	}

	if len(response.Choices) > 0 {
		newResponse := response.Choices[0].Message.Content

		// Annotate the last frame with bounding boxes
		finalFrame := framesToSend[len(framesToSend)-1]
		annotatedData, err := AnnotateFrame(finalFrame.Data, newResponse)
		if err != nil {
			log.Printf("[BatchManager] Failed to annotate frame: %v", err)
			annotatedData = finalFrame.Data // fall back to original
		}

		// Save annotated frame to disk (replaces the preprocessed-only version)
		// This goes to the same storage/frames/{serviceID}/ directory
		if m.storage != nil {
			annotatedFrame := &Frame{
				Data:      annotatedData,
				Timestamp: finalFrame.Timestamp,
				ServiceID: serviceID,
			}
			m.storage.Save(serviceID, annotatedFrame)
		}

		// Update context with annotated frame for next batch's continuity
		// This is a "set of marks" approach - the model sees previous detections as visual markers
		// Only update if this batch is newer than the current context (prevents out-of-order updates)
		m.mu.Lock()
		if existingCtx := m.rollingContexts[serviceID]; existingCtx != nil {
			if batchSentTime >= existingCtx.lastBatchSentTime {
				existingCtx.previousResponse = newResponse
				existingCtx.lastFrame = &Frame{
					Data:      annotatedData, // Annotated frame with bounding boxes as visual markers
					Timestamp: finalFrame.Timestamp,
					ServiceID: serviceID,
				}
				existingCtx.lastBatchSentTime = batchSentTime
			} else {
				log.Printf("[BatchManager] Ignoring out-of-order response for service %s (batch sent at %d, current context at %d)",
					serviceID, batchSentTime, existingCtx.lastBatchSentTime)
			}
		}
		m.mu.Unlock()

		log.Printf("[BatchManager] Service %s: annotated frame", serviceID)
	}
}

// Flush sends all pending batches regardless of size
func (m *BatchManager) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for serviceID, batch := range m.frameBatches {
		if len(batch) > 0 {
			framesToSend := make([]*Frame, len(batch))
			copy(framesToSend, batch)
			m.frameBatches[serviceID] = nil

			// Get or create context
			rollCtx := m.rollingContexts[serviceID]
			if rollCtx == nil {
				rollCtx = &rollingContext{}
				m.rollingContexts[serviceID] = rollCtx
			}

			// Store last frame and capture send timestamp
			newLastFrame := framesToSend[len(framesToSend)-1]
			batchSentTime := time.Now().UnixNano()

			// Snapshot context data for async goroutine
			prevResponse := rollCtx.previousResponse
			prevLastFrame := rollCtx.lastFrame

			// Update context
			rollCtx.lastFrame = newLastFrame
			rollCtx.lastBatchSentTime = batchSentTime

			go m.sendBatch(serviceID, framesToSend, prevResponse, prevLastFrame, batchSentTime)
		}
	}
}

// GetSummary returns the current rolling summary for a service
func (m *BatchManager) GetSummary(serviceID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rollCtx := m.rollingContexts[serviceID]; rollCtx != nil {
		return rollCtx.previousResponse
	}
	return ""
}

// GetContext returns a copy of the rolling context for a service
// Returns nil if no context exists for the service
func (m *BatchManager) GetContext(serviceID string) *rollingContext {
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx := m.rollingContexts[serviceID]
	if ctx == nil {
		return nil
	}
	// Return a copy to prevent external mutation
	return &rollingContext{
		lastFrame:         ctx.lastFrame,
		previousResponse:  ctx.previousResponse,
		lastBatchSentTime: ctx.lastBatchSentTime,
	}
}

// ResetSummary clears the rolling context for a service
func (m *BatchManager) ResetSummary(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rollingContexts, serviceID)
	log.Printf("[BatchManager] Reset rolling context for service %s", serviceID)
}

// RemoveService completely removes all data for a service (frames and context)
// Call this when a service is permanently deleted
func (m *BatchManager) RemoveService(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.frameBatches, serviceID)
	delete(m.rollingContexts, serviceID)
	log.Printf("[BatchManager] Removed service %s (freed batches and context)", serviceID)
}
