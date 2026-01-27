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
	previousResponse string  // Last VLM raw response (JSON)
	effectiveWidth   int     // Model's effective image width (from models.Cache)
	effectiveHeight  int     // Model's effective image height
}

// BatchManager accumulates frames and sends them in batches to vLLM
// It maintains rolling context per service, asking the model to describe only what's new
type BatchManager struct {
	client          *FrameClient
	batchSize       int
	frameBatches    map[string][]*Frame     // serviceID -> pending frames
	rollingContexts map[string]*rollingContext // serviceID -> rolling state
	baseInstruction string                   // Base instruction
	storageBaseDir  string                   // Base directory for storing annotated frames
	mu              sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int, storageBaseDir string) *BatchManager {
	return &BatchManager{
		client:           client,
		batchSize:        batchSize,
		frameBatches:     make(map[string][]*Frame),
		rollingContexts:  make(map[string]*rollingContext),
		baseInstruction:  "Analyze these video frames for motion, action, emotion, facial expressions, and subtle details. Describe what you observe and judge the significance of changes.",
		storageBaseDir:   storageBaseDir,
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
	// Build instruction based on whether we have previous context
	// Note: JSON schema is handled by response_format in FrameClient
	instruction := m.baseInstruction

	if rollCtx.previousResponse != "" {
		instruction = strings.Join([]string{
			m.baseInstruction,
			"",
			"PREVIOUS ANALYSIS (use for object tracking):",
			rollCtx.previousResponse,
			"",
			"Detect ALL objects in the current frame. For objects that match previous objects (same type, similar location), use their existing ID.",
			"For new objects, assign new IDs (continue numbering from previous).",
			"Provide complete descriptions with accurate positions.",
			"Focus on motion, action, emotion, facial expressions, and subtle details.",
			"In the description, reference objects by their IDs in brackets, e.g., 'The person [3] is driving a red car [4]'.",
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

		// Get effective dimensions from model cache
		effectiveWidth, effectiveHeight := 1280, 720 // fallback defaults
		if m.client.ModelCache != nil {
			if info, err := m.client.ModelCache.GetModelInfo(m.client.Model); err == nil {
				if info.EffectiveWidth != nil && info.EffectiveHeight != nil {
					effectiveWidth, effectiveHeight = *info.EffectiveWidth, *info.EffectiveHeight
				}
			}
		}

		// Annotate the last frame with bounding boxes
		lastFrame := framesToSend[len(framesToSend)-1]
		annotatedData, err := AnnotateFrame(lastFrame.Data, newResponse, effectiveWidth, effectiveHeight)
		if err != nil {
			log.Printf("[BatchManager] Failed to annotate frame: %v", err)
			annotatedData = lastFrame.Data // fall back to original
		}

		// Save annotated frame to disk for debugging
		go SaveAnnotatedFrame(annotatedData, serviceID, lastFrame.Sequence, m.storageBaseDir)

		// Update context with annotated frame for next batch's continuity
		m.mu.Lock()
		if existingCtx := m.rollingContexts[serviceID]; existingCtx != nil {
			existingCtx.previousResponse = newResponse
			existingCtx.lastFrame = &Frame{
				Data:      annotatedData,
				Timestamp: lastFrame.Timestamp,
				ServiceID: serviceID,
				Sequence:  lastFrame.Sequence,
			}
			existingCtx.effectiveWidth = effectiveWidth
			existingCtx.effectiveHeight = effectiveHeight
		}
		m.mu.Unlock()

		log.Printf("[BatchManager] Service %s: annotated frame (effective: %dx%d)",
			serviceID, effectiveWidth, effectiveHeight)
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
