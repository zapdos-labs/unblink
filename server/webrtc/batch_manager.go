package webrtc

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	servicev1 "unblink/server/gen/service/v1"
	"unblink/database"
	"unblink/server/internal/timeutil"
)

// EventBroadcaster interface for broadcasting events (avoid circular dependency)
type EventBroadcaster interface {
	Broadcast(event *servicev1.Event, nodeID string)
}

// rollingContext holds the rolling state for a service
type rollingContext struct {
	lastFrame         *Frame       // Last frame from previous batch
	previousResponse  *VLMResponse // Last VLM parsed response (full object with description and objects)
	lastBatchSentTime int64        // Unix nano timestamp of last batch sent (for ordering)
}

// BatchManager accumulates frames and sends them in batches to vLLM
// It maintains rolling context per service, asking the model to describe only what's new
type BatchManager struct {
	client            *FrameClient
	batchSize         int
	frameBuffers      map[string][]*Frame        // serviceID -> buffer of frames (max size = batchSize)
	rollingContexts   map[string]*rollingContext // serviceID -> rolling state
	processingLocks   map[string]bool            // serviceID -> is currently processing
	baseInstruction   string                     // Base instruction
	storage           *Storage                   // Storage for saving annotated frames
	db                *database.Client           // Database for events
	eventBroadcaster  EventBroadcaster           // For broadcasting events to subscribers
	mu                sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int, storage *Storage, db *database.Client, broadcaster EventBroadcaster) *BatchManager {
	return &BatchManager{
		client:           client,
		batchSize:        batchSize,
		frameBuffers:     make(map[string][]*Frame),
		rollingContexts:  make(map[string]*rollingContext),
		processingLocks:  make(map[string]bool),
		baseInstruction:  "Analyze these video frames for motion, action, emotion, facial expressions, and subtle details. Detect the most important objects (MAX 10) and return bounding boxes in NORMALIZED 1000 COORDINATES (0=top/left, 1000=bottom/right).",
		storage:          storage,
		db:               db,
		eventBroadcaster: broadcaster,
	}
}

// AddFrame adds a frame to the buffer
// If processing: add to buffer, remove oldest if buffer full
// If not processing: grab all frames in buffer and send
func (m *BatchManager) AddFrame(frame *Frame) {
	m.mu.Lock()

	serviceID := frame.ServiceID

	// Initialize buffer if needed
	if m.frameBuffers[serviceID] == nil {
		m.frameBuffers[serviceID] = make([]*Frame, 0, m.batchSize)
	}

	buffer := m.frameBuffers[serviceID]

	// Add frame to buffer
	buffer = append(buffer, frame)

	// If buffer exceeds batch size, remove oldest frame (FIFO)
	if len(buffer) > m.batchSize {
		buffer = buffer[1:] // Remove first (oldest) element
		log.Printf("[BatchManager] Buffer full for service %s, removed oldest frame", serviceID)
	}

	m.frameBuffers[serviceID] = buffer

	// Check if we should send a batch:
	// 1. Buffer is full (== batchSize)
	// 2. This service is not currently processing
	shouldSend := len(buffer) == m.batchSize && !m.processingLocks[serviceID]

	if shouldSend {
		// Grab all frames from buffer
		framesToSend := make([]*Frame, len(buffer))
		copy(framesToSend, buffer)

		// Clear the buffer
		m.frameBuffers[serviceID] = nil

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

		// Mark service as processing
		m.processingLocks[serviceID] = true

		m.mu.Unlock()

		// Send batch asynchronously to avoid blocking
		go m.sendBatch(serviceID, framesToSend, prevResponse, prevLastFrame, batchSentTime)
	} else {
		if m.processingLocks[serviceID] {
			log.Printf("[BatchManager] Buffering frame for service %s (currently processing, %d frames in buffer)", serviceID, len(buffer))
		}
		m.mu.Unlock()
	}
}

// sendBatch sends a batch of frames to vLLM with rolling context
func (m *BatchManager) sendBatch(serviceID string, frames []*Frame, previousResponse *VLMResponse, lastFrame *Frame, batchSentTime int64) {
	// Release the processing lock when done
	defer func() {
		m.mu.Lock()
		delete(m.processingLocks, serviceID)
		m.mu.Unlock()
	}()
	// Build instruction based on whether we have previous context
	// Note: JSON schema is handled by response_format in FrameClient
	instruction := m.baseInstruction

	if previousResponse != nil && previousResponse.Description != "" {
		instruction = strings.Join([]string{
			m.baseInstruction,
			"",
			"PREVIOUS ANALYSIS (use for object tracking):",
			previousResponse.Description,
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
		newResponseStr := response.Choices[0].Message.Content

		// Parse VLM response into structured format
		var vlmResp VLMResponse
		if err := json.Unmarshal([]byte(newResponseStr), &vlmResp); err != nil {
			log.Printf("[BatchManager] Failed to parse VLM response: %v", err)
			return
		}

		// Create event for VLM indexing with structured data
		if m.db != nil {
			// Convert VLMResponse struct to map via JSON round-trip for structpb compatibility
			var responseMap map[string]any
			respJSON, err := json.Marshal(vlmResp)
			if err != nil {
				log.Printf("[BatchManager] Failed to marshal VLM response: %v", err)
			} else if err := json.Unmarshal(respJSON, &responseMap); err != nil {
				log.Printf("[BatchManager] Failed to unmarshal VLM response to map: %v", err)
			} else {
				// Calculate time span and granularity
				firstFrame := framesToSend[0]
				lastFrame := framesToSend[len(framesToSend)-1]
				duration := lastFrame.Timestamp.Sub(firstFrame.Timestamp)
				granularity := timeutil.CalculateGranularity(int64(duration.Seconds()))

				payload, err := structpb.NewStruct(map[string]any{
					"type":       "vlm-indexing",
					"granularity": string(granularity),
					"from_iso":   timeutil.FormatToISO(firstFrame.Timestamp),
					"to_iso":     timeutil.FormatToISO(lastFrame.Timestamp),
					"response":   responseMap,
				})
				if err != nil {
					log.Printf("[BatchManager] Failed to create event payload: %v", err)
				} else {
					eventID := uuid.New().String()
					if err := m.db.CreateEvent(eventID, serviceID, payload); err != nil {
						log.Printf("[BatchManager] Failed to create event: %v", err)
					} else {
						// Broadcast the event to subscribers
						if m.eventBroadcaster != nil {
							event := &servicev1.Event{
								Id:        eventID,
								ServiceId: serviceID,
								Payload:   payload,
								CreatedAt: timestamppb.New(time.Now()),
							}
							// Get node_id from service
							if svc, err := m.db.GetService(serviceID); err == nil && svc != nil {
								m.eventBroadcaster.Broadcast(event, svc.NodeId)
							}
						}
					}
				}
			}
		}

		// Annotate the last frame with bounding boxes
		finalFrame := framesToSend[len(framesToSend)-1]
		annotatedData, err := AnnotateFrame(finalFrame.Data, newResponseStr)
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
				// Store the full VLM response for next batch context
				existingCtx.previousResponse = &vlmResp
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

		log.Printf("[BatchManager] Service %s: annotated frame with %d objects", serviceID, len(vlmResp.Objects))
	}
}

// Flush sends batches from buffers regardless of batch size
// Grabs all available frames from each service's buffer
func (m *BatchManager) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for serviceID, buffer := range m.frameBuffers {
		if len(buffer) > 0 && !m.processingLocks[serviceID] {
			framesToSend := make([]*Frame, len(buffer))
			copy(framesToSend, buffer)

			// Clear buffer
			m.frameBuffers[serviceID] = nil

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

			// Mark service as processing
			m.processingLocks[serviceID] = true

			go m.sendBatch(serviceID, framesToSend, prevResponse, prevLastFrame, batchSentTime)
		}
	}
}

// GetSummary returns the current rolling summary for a service
func (m *BatchManager) GetSummary(serviceID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rollCtx := m.rollingContexts[serviceID]; rollCtx != nil && rollCtx.previousResponse != nil {
		return rollCtx.previousResponse.Description
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
	delete(m.frameBuffers, serviceID)
	delete(m.rollingContexts, serviceID)
	delete(m.processingLocks, serviceID)
	log.Printf("[BatchManager] Removed service %s (freed buffer and context)", serviceID)
}
