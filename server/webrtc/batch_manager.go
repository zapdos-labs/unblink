package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zapdos-labs/unblink/database"
	servicev1 "github.com/zapdos-labs/unblink/server/gen/service/v1"
	"github.com/zapdos-labs/unblink/server/internal/timeutil"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// rollupWindow tracks the current accumulation window for lower granularity events.
type rollupWindow struct {
	from time.Time
	to   time.Time
}

// BatchManager accumulates frames and sends them in batches to vLLM
// It maintains rolling context per service, asking the model to describe only what's new
type BatchManager struct {
	client           *FrameClient
	batchSize        int
	frameBuffers     map[string][]*Frame        // serviceID -> buffer of frames (max size = batchSize)
	rollingContexts  map[string]*rollingContext // serviceID -> rolling state
	rollupWindows    map[string]map[timeutil.GranularityLevel]*rollupWindow
	processingLocks  map[string]bool  // serviceID -> is currently processing
	baseInstruction  string           // Base instruction
	storage          *Storage         // Storage for saving annotated frames
	db               *database.Client // Database for events
	eventBroadcaster EventBroadcaster // For broadcasting events to subscribers
	mu               sync.Mutex
}

// NewBatchManager creates a new batch manager
func NewBatchManager(client *FrameClient, batchSize int, storage *Storage, db *database.Client, broadcaster EventBroadcaster) *BatchManager {
	return &BatchManager{
		client:          client,
		batchSize:       batchSize,
		frameBuffers:    make(map[string][]*Frame),
		rollingContexts: make(map[string]*rollingContext),
		rollupWindows:   make(map[string]map[timeutil.GranularityLevel]*rollupWindow),
		processingLocks: make(map[string]bool),
		baseInstruction: strings.Join([]string{
			"Analyze these video frames for motion, action, emotion, facial expressions, and subtle details.",
			"Detect the most important objects (MAX 10) and return bounding boxes in NORMALIZED 1000 COORDINATES (0=top/left, 1000=bottom/right).",
			"",
			"USE CASE PRIORITIES:",
			"1. Detect when any worker is trying to open a container while the container has no visible color safety belt. Treat this as an SOP violation and clearly call out that it should alert.",
			"2. Detect when any worker is dangerously close to a forklift, especially if the worker is within the red laser boundary or roughly 1-2 meters from the forklift. Treat this as an unsafe proximity event and clearly call out that it should alert.",
		}, "\n"),
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
				// Base events are emitted from each frame batch.
				firstFrame := framesToSend[0]
				lastFrame := framesToSend[len(framesToSend)-1]
				duration := lastFrame.Timestamp.Sub(firstFrame.Timestamp)
				granularity := timeutil.CalculateGranularity(int64(duration.Seconds()))

				if _, err := m.createAndBroadcastEvent(serviceID, granularity, firstFrame.Timestamp, lastFrame.Timestamp, responseMap); err != nil {
					log.Printf("[BatchManager] Failed to create %s event: %v", granularity, err)
				} else {
					m.maybeBuildHigherGranularity(serviceID, granularity, firstFrame.Timestamp, lastFrame.Timestamp)
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

func (m *BatchManager) createAndBroadcastEvent(serviceID string, granularity timeutil.GranularityLevel, from, to time.Time, responseMap map[string]any) (*servicev1.Event, error) {
	payload, err := structpb.NewStruct(map[string]any{
		"type":        "vlm-indexing",
		"granularity": string(granularity),
		"from_iso":    timeutil.FormatToISO(from),
		"to_iso":      timeutil.FormatToISO(to),
		"response":    responseMap,
	})
	if err != nil {
		return nil, fmt.Errorf("create payload: %w", err)
	}

	eventID := uuid.New().String()
	if err := m.db.CreateEvent(eventID, serviceID, payload); err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	event := &servicev1.Event{
		Id:        eventID,
		ServiceId: serviceID,
		Payload:   payload,
		CreatedAt: timestamppb.New(time.Now()),
	}
	if m.eventBroadcaster != nil {
		if svc, err := m.db.GetService(serviceID); err == nil && svc != nil {
			m.eventBroadcaster.Broadcast(event, svc.NodeId)
		}
	}
	return event, nil
}

// maybeBuildHigherGranularity accumulates lower-level event time in memory and, once threshold is reached,
// loads lower-level events from DB, summarizes them via VLM, and emits a higher-level event.
func (m *BatchManager) maybeBuildHigherGranularity(serviceID string, lowerGranularity timeutil.GranularityLevel, lowerFrom, lowerTo time.Time) {
	nextGranularity, ok := timeutil.NextGranularity(lowerGranularity)
	if !ok || m.db == nil {
		return
	}

	windowFrom, windowTo, ready := m.extendRollupWindow(serviceID, lowerGranularity, lowerFrom, lowerTo, timeutil.MinSecondsForGranularity(nextGranularity))
	if !ready {
		return
	}

	lowerEvents, err := m.db.ListVLMEventsByGranularityRange(serviceID, string(lowerGranularity), windowFrom, windowTo)
	if err != nil {
		log.Printf("[BatchManager] Failed loading %s events for %s rollup: %v", lowerGranularity, nextGranularity, err)
		return
	}
	if len(lowerEvents) == 0 {
		return
	}

	aggregatedResp, err := m.aggregateLowerEventsWithVLM(serviceID, lowerGranularity, nextGranularity, windowFrom, windowTo, lowerEvents)
	if err != nil {
		log.Printf("[BatchManager] Failed aggregating %s -> %s: %v", lowerGranularity, nextGranularity, err)
		return
	}

	if _, err := m.createAndBroadcastEvent(serviceID, nextGranularity, windowFrom, windowTo, aggregatedResp); err != nil {
		log.Printf("[BatchManager] Failed to create %s rollup event: %v", nextGranularity, err)
		return
	}
	m.clearRollupWindow(serviceID, lowerGranularity)

	// Feed the produced higher-level event into the next rollup tier.
	m.maybeBuildHigherGranularity(serviceID, nextGranularity, windowFrom, windowTo)
}

// extendRollupWindow extends or initializes the rollup window for one service + lower granularity.
// It returns ready=true when the accumulated span reaches thresholdSeconds.
func (m *BatchManager) extendRollupWindow(serviceID string, lowerGranularity timeutil.GranularityLevel, batchFrom, batchTo time.Time, thresholdSeconds int64) (time.Time, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	windowsByGranularity := m.rollupWindows[serviceID]
	if windowsByGranularity == nil {
		windowsByGranularity = make(map[timeutil.GranularityLevel]*rollupWindow)
		m.rollupWindows[serviceID] = windowsByGranularity
	}

	win := windowsByGranularity[lowerGranularity]
	if win == nil {
		win = &rollupWindow{
			from: batchFrom,
			to:   batchTo,
		}
		windowsByGranularity[lowerGranularity] = win
	} else {
		if batchFrom.Before(win.from) {
			win.from = batchFrom
		}
		if batchTo.After(win.to) {
			win.to = batchTo
		}
	}

	if win.to.Sub(win.from) < time.Duration(thresholdSeconds)*time.Second {
		return win.from, win.to, false
	}

	return win.from, win.to, true
}

func (m *BatchManager) clearRollupWindow(serviceID string, granularity timeutil.GranularityLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	windowsByGranularity := m.rollupWindows[serviceID]
	if windowsByGranularity == nil {
		return
	}
	delete(windowsByGranularity, granularity)
	if len(windowsByGranularity) == 0 {
		delete(m.rollupWindows, serviceID)
	}
}

func (m *BatchManager) aggregateLowerEventsWithVLM(serviceID string, lowerGranularity, nextGranularity timeutil.GranularityLevel, from, to time.Time, lowerEvents []*servicev1.Event) (map[string]any, error) {
	sourceRows := make([]map[string]any, 0, len(lowerEvents))
	for _, ev := range lowerEvents {
		if ev.GetPayload() == nil {
			continue
		}
		payloadMap := ev.Payload.AsMap()
		sourceRows = append(sourceRows, map[string]any{
			"id":          ev.GetId(),
			"from_iso":    payloadMap["from_iso"],
			"to_iso":      payloadMap["to_iso"],
			"granularity": payloadMap["granularity"],
			"response":    payloadMap["response"],
		})
	}
	if len(sourceRows) == 0 {
		return nil, fmt.Errorf("no lower-level payloads available")
	}

	sourceJSON, err := json.Marshal(sourceRows)
	if err != nil {
		return nil, fmt.Errorf("marshal source events: %w", err)
	}

	instruction := strings.Join([]string{
		fmt.Sprintf("Aggregate lower-granularity camera events into one %s-level event.", nextGranularity),
		fmt.Sprintf("Input events are %s-level and already structured.", lowerGranularity),
		"Return consolidated objects and one concise, accurate description for the full time window.",
		"If details conflict, prefer consistency with majority evidence across events.",
		"Do not invent objects or actions not present in input events.",
		"Output must follow the provided JSON schema exactly.",
	}, "\n")

	textInput := strings.Join([]string{
		fmt.Sprintf("service_id: %s", serviceID),
		fmt.Sprintf("from_iso: %s", timeutil.FormatToISO(from)),
		fmt.Sprintf("to_iso: %s", timeutil.FormatToISO(to)),
		fmt.Sprintf("source_count: %d", len(sourceRows)),
		"source_events_json:",
		string(sourceJSON),
	}, "\n")

	resp, err := m.client.SendTextWithStructuredOutput(context.Background(), instruction, textInput)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty aggregation response")
	}

	var summarized VLMResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &summarized); err != nil {
		return nil, fmt.Errorf("parse aggregation response: %w", err)
	}

	respJSON, err := json.Marshal(summarized)
	if err != nil {
		return nil, fmt.Errorf("marshal summarized response: %w", err)
	}

	var responseMap map[string]any
	if err := json.Unmarshal(respJSON, &responseMap); err != nil {
		return nil, fmt.Errorf("convert summarized response to map: %w", err)
	}
	return responseMap, nil
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
	delete(m.rollupWindows, serviceID)
	log.Printf("[BatchManager] Reset rolling context for service %s", serviceID)
}

// RemoveService completely removes all data for a service (frames and context)
// Call this when a service is permanently deleted
func (m *BatchManager) RemoveService(serviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.frameBuffers, serviceID)
	delete(m.rollingContexts, serviceID)
	delete(m.rollupWindows, serviceID)
	delete(m.processingLocks, serviceID)
	log.Printf("[BatchManager] Removed service %s (freed buffer and context)", serviceID)
}
