package webrtc

import (
	"context"
	"log"
	"strings"
	"sync"
)

// rollingContext holds the rolling state for a service
type rollingContext struct {
	lastFrame        *Frame // Last frame from previous batch
	previousResponse string  // Last VLM response
}

// BatchManager accumulates frames and sends them in batches to vLLM
// It maintains rolling context per service, asking the model to describe only what's new
type BatchManager struct {
	client          *FrameClient
	batchSize       int
	frameBatches    map[string][]*Frame  // serviceID -> pending frames
	rollingContexts map[string]*rollingContext // serviceID -> rolling state
	baseInstruction string                // Base instruction
	mu              sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int) *BatchManager {
	return &BatchManager{
		client:           client,
		batchSize:        batchSize,
		frameBatches:     make(map[string][]*Frame),
		rollingContexts:  make(map[string]*rollingContext),
		baseInstruction:  "Describe what is visible in these video frames.",
	}
}

// AddFrame adds a frame to the batch and sends if batch is full
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

		// Store last frame for next batch
		rollCtx.lastFrame = framesToSend[len(framesToSend)-1]

		m.mu.Unlock()

		// Send batch (outside lock)
		m.sendBatch(serviceID, framesToSend, rollCtx)
	} else {
		m.mu.Unlock()
	}
}

// sendBatch sends a batch of frames to vLLM with rolling context
func (m *BatchManager) sendBatch(serviceID string, frames []*Frame, rollCtx *rollingContext) {
	// Build instruction: ask for only what's new if we have previous context
	instruction := m.baseInstruction
	if rollCtx.previousResponse != "" && rollCtx.lastFrame != nil {
		instruction = strings.Join([]string{
			m.baseInstruction,
			"",
			"PREVIOUS SUMMARY (what was already described):",
			rollCtx.previousResponse,
			"",
			"IMPORTANT: Only describe what is NEW or has CHANGED since the previous summary.",
			"Do not repeat things that were already mentioned.",
			"Focus on actions, movements, or scene changes.",
			"",
			"Note: The first frame shown is the LAST frame from the previous batch,",
			"provided for visual continuity. Focus your description on what's different",
			"in the subsequent frames compared to that first frame.",
		}, "\n")
	}

	// Prepend last frame for visual continuity
	framesToSend := frames
	if rollCtx.lastFrame != nil {
		framesToSend = append([]*Frame{rollCtx.lastFrame}, frames...)
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

		// Update context
		m.mu.Lock()
		if existingCtx := m.rollingContexts[serviceID]; existingCtx != nil {
			existingCtx.previousResponse = newResponse
		}
		m.mu.Unlock()

		log.Printf("[BatchManager] Service %s rolling summary updated:\n%s", serviceID, newResponse)
		// TODO: Store the summary somewhere (database, broadcast to clients, etc.)
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

			// Store last frame
			rollCtx.lastFrame = framesToSend[len(framesToSend)-1]

			go m.sendBatch(serviceID, framesToSend, rollCtx)
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

// GetContext returns the full rolling context for a service
func (m *BatchManager) GetContext(serviceID string) *rollingContext {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rollingContexts[serviceID]
}

// ResetSummary clears the rolling context for a service
func (m *BatchManager) ResetSummary(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rollingContexts, serviceID)
	log.Printf("[BatchManager] Reset rolling context for service %s", serviceID)
}
