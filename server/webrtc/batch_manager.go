package webrtc

import (
	"context"
	"log"
	"sync"
)

// BatchManager accumulates frames and sends them in batches to vLLM
type BatchManager struct {
	client       *FrameClient
	batchSize    int
	frameBatches map[string][]*Frame // serviceID -> frames
	mu           sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int) *BatchManager {
	return &BatchManager{
		client:       client,
		batchSize:    batchSize,
		frameBatches: make(map[string][]*Frame),
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
		m.mu.Unlock()

		// Send batch (outside lock)
		m.sendBatch(serviceID, framesToSend)
	} else {
		m.mu.Unlock()
	}
}

// sendBatch sends a batch of frames to vLLM
func (m *BatchManager) sendBatch(serviceID string, frames []*Frame) {
	log.Printf("[BatchManager] Sending batch for service %s: %d frames", serviceID, len(frames))

	ctx := context.Background()
	response, err := m.client.SendFrameBatch(ctx, frames)
	if err != nil {
		log.Printf("[BatchManager] Failed to send batch for service %s: %v", serviceID, err)
		return
	}

	if len(response.Choices) > 0 {
		log.Printf("[BatchManager] Service %s summary: %s", serviceID, response.Choices[0].Message.Content)
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

			go m.sendBatch(serviceID, framesToSend)
		}
	}
}
