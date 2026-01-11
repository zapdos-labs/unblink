package cv

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/google/uuid"
)

// RealtimeStream interface for dependency injection
type RealtimeStream interface {
	GetReceivers() []*core.Receiver
	GetProducer() core.Producer
	GetService() interface{} // Returns node.Service
	GetID() string
	CloseChan() <-chan struct{}
}

// CVFrameExtractor extracts JPEG frames from H.264 streams
type CVFrameExtractor struct {
	serviceID       string
	extractInterval time.Duration
	eventBus        *CVEventBus
	storageManager  *StorageManager
	closeChan       chan struct{}

	// FFmpeg pipeline
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser
	frameReady   chan []byte
	mu           sync.Mutex

	// Length-prefix buffering for split NAL units
	lengthPrefixBuffer []byte
	// Tracks if we're in length-prefix mode (persists across packets)
	isLengthPrefixMode bool

	// Frame batching for batch events
	frameBatch     []string
	batchSize      int
	batchStartTime time.Time
}

// NewCVFrameExtractor creates a new frame extractor
func NewCVFrameExtractor(serviceID string, extractInterval time.Duration, eventBus *CVEventBus, storageManager *StorageManager, batchSize int) *CVFrameExtractor {
	return &CVFrameExtractor{
		serviceID:       serviceID,
		extractInterval: extractInterval,
		eventBus:        eventBus,
		storageManager:  storageManager,
		closeChan:       make(chan struct{}),
		frameReady:      make(chan []byte, 10), // Buffer a few frames
		frameBatch:      make([]string, 0, batchSize),
		batchSize:       batchSize,
	}
}

// Start begins extracting frames from the realtime stream
func (e *CVFrameExtractor) Start(stream RealtimeStream) error {
	log.Printf("[CVFrameExtractor] Starting frame extraction for service %s (interval=%v)", e.serviceID, e.extractInterval)

	// Start FFmpeg process
	if err := e.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Start H.264 packet consumer that pipes to FFmpeg
	producer := stream.GetProducer()
	go e.consumeH264ToFFmpeg(producer)

	// Start JPEG frame reader from FFmpeg
	go e.readFramesFromFFmpeg()

	// Start frame saver loop
	go e.saveFrameLoop()

	return nil
}

// startFFmpeg starts the FFmpeg process for H.264 to JPEG conversion
func (e *CVFrameExtractor) startFFmpeg() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate fps from interval (e.g., 5s interval = 1/5 fps = 0.2 fps)
	fps := 1.0 / e.extractInterval.Seconds()

	// FFmpeg command: H.264 stdin → JPEG frames at specified fps
	e.ffmpegCmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error", // Only log errors, suppress progress info
		"-f", "h264", // Input format
		"-i", "pipe:0", // Read from stdin
		"-vf", fmt.Sprintf("fps=%.3f", fps), // Extract frames at interval
		"-f", "image2pipe", // Output image stream
		"-c:v", "mjpeg", // JPEG codec
		"-q:v", "2", // Quality (1-31, lower is better)
		"pipe:1", // Write to stdout
	)

	var err error
	e.ffmpegStdin, err = e.ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	e.ffmpegStdout, err = e.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	e.ffmpegCmd.Stderr = os.Stderr

	if err := e.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	log.Printf("[CVFrameExtractor] Started FFmpeg for service %s (fps=%.3f)", e.serviceID, fps)
	return nil
}

// consumeH264ToFFmpeg reads H.264 packets and pipes them to FFmpeg
func (e *CVFrameExtractor) consumeH264ToFFmpeg(producer core.Producer) {
	defer log.Printf("[CVFrameExtractor] Stopped H.264 consumer for service %s", e.serviceID)
	defer func() {
		if e.ffmpegStdin != nil {
			e.ffmpegStdin.Close()
		}
	}()

	// Get video media (H.264)
	var videoMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindVideo {
			videoMedia = media
			break
		}
	}

	if videoMedia == nil {
		log.Printf("[CVFrameExtractor] No video media found for service %s", e.serviceID)
		return
	}

	// Get H.264 codec
	var videoCodec *core.Codec
	for _, codec := range videoMedia.Codecs {
		if codec.Name == core.CodecH264 {
			videoCodec = codec
			break
		}
	}

	if videoCodec == nil {
		log.Printf("[CVFrameExtractor] No H.264 codec found for service %s", e.serviceID)
		return
	}

	// Get track (receiver) for the video codec
	receiver, err := producer.GetTrack(videoMedia, videoCodec)
	if err != nil {
		log.Printf("[CVFrameExtractor] Failed to get track for service %s: %v", e.serviceID, err)
		return
	}

	// Create a sender that will pipe packets to FFmpeg
	sender := core.NewSender(videoMedia, videoCodec)

	// Annex B start code
	startCode := []byte{0x00, 0x00, 0x00, 0x01}

	// Buffer for FU-A fragment reassembly
	var fuBuffer []byte
	var fuStarted bool

	// Debug: packet counter
	pktCount := 0

	// Set handler to write H.264 NAL units to FFmpeg
	sender.Handler = func(pkt *core.Packet) {
		if e.ffmpegStdin == nil {
			return
		}

		// pkt.Payload is the RTP payload (H.264 data per RFC 6184)
		// OR it could be raw Annex B data (already has start codes)
		payload := pkt.Payload
		if len(payload) == 0 {
			return
		}

		// Detect format:
		// - Annex B: starts with 0x00 0x00 0x00 0x01
		// - Length-prefix: starts with 0x00 0x00 0x00 XX (where XX != 0x01)
		// - RTP: first byte has valid NAL type (1-28)
		isAnnexB := len(payload) >= 4 && payload[0] == 0x00 && payload[1] == 0x00 && payload[2] == 0x00 && payload[3] == 0x01
		isLengthPrefix := !isAnnexB && len(payload) >= 4 && payload[0] == 0x00 && payload[1] == 0x00 && payload[2] == 0x00 && payload[3] != 0x01

		pktCount++

		// If we have buffered data from previous incomplete NAL unit, prepend it
		// We only buffer in length-prefix mode, so we know this is length-prefix data
		if len(e.lengthPrefixBuffer) > 0 {
			payload = append(e.lengthPrefixBuffer, payload...)
			e.lengthPrefixBuffer = nil
			isLengthPrefix = true
			isAnnexB = false
		}

		// If we detected length-prefix format in a previous packet, stay in that mode
		// This handles continuation packets that don't start with the length-prefix signature
		if e.isLengthPrefixMode && !isAnnexB && !isLengthPrefix {
			isLengthPrefix = true
		}

		// Remember length-prefix mode for continuation packets
		if isLengthPrefix {
			e.isLengthPrefixMode = true
		} else if isAnnexB {
			e.isLengthPrefixMode = false
		}

		// If Annex B format, write directly
		if isAnnexB {
			e.ffmpegStdin.Write(payload)
			return
		}

		// If length-prefix format (from magic.Open), convert to Annex B
		if isLengthPrefix {
			// payload already has buffered data combined (if any)
			data := payload
			offset := 0
			nalUnitCount := 0
			for offset < len(data) {
				// Need at least 4 bytes for size
				if offset+4 > len(data) {
					// Save remaining data for next packet
					e.lengthPrefixBuffer = data[offset:]
					return
				}
				// Read 4-byte big-endian size
				nalLen := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
				offset += 4

				// Check if we have enough data
				if offset+nalLen > len(data) {
					// Save incomplete NAL unit for next packet
					e.lengthPrefixBuffer = data[offset-4:] // Include the 4-byte length prefix
					return
				}

				// Write start code + NAL unit
				e.ffmpegStdin.Write(startCode)
				e.ffmpegStdin.Write(data[offset : offset+nalLen])
				offset += nalLen
				nalUnitCount++
			}
			return
		}

		// Otherwise, process as RTP payload
		// H.264 RTP payload types (RFC 6184)
		nalType := payload[0] & 0x1F

		switch nalType {
		case 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23: // Single NAL unit
			// Write start code + NAL unit
			e.ffmpegStdin.Write(startCode)
			e.ffmpegStdin.Write(payload)

		case 24: // STAP-A (single-time aggregation packet)
			// Contains multiple NAL units, each prefixed with length
			offset := 1 // Skip STAP-A header
			for offset < len(payload) {
				if offset+2 > len(payload) {
					break
				}
				nalLen := int(payload[offset])<<8 | int(payload[offset+1])
				offset += 2
				if offset+nalLen > len(payload) {
					break
				}
				// Write each NAL unit with start code
				e.ffmpegStdin.Write(startCode)
				e.ffmpegStdin.Write(payload[offset : offset+nalLen])
				offset += nalLen
			}

		case 28: // FU-A (fragmentation unit)
			if len(payload) < 2 {
				return
			}
			// FU indicator and FU header structure:
			//   byte 0: FU indicator (nal_ref_idc << 5 | 28)
			//   byte 1: FU header (start<<7 | end<<6 | reserved<<6 | nal_type)
			isStart := (payload[1] & 0x80) != 0
			isEnd := (payload[1] & 0x40) != 0
			originalNalType := payload[1] & 0x1F

			if isStart {
				// Start fragment: reassemble original NAL header
				// Original NAL header = (payload[0] & 0xE0) | originalNalType
				nalHeader := (payload[0] & 0xE0) | originalNalType
				fuBuffer = append([]byte{nalHeader}, payload[2:]...)
				fuStarted = true
			} else if isEnd {
				// End fragment: complete the NAL unit and write
				if !fuStarted {
					// Got end without start - corrupted stream, discard
					fuBuffer = nil
					fuStarted = false
					return
				}
				fuBuffer = append(fuBuffer, payload[2:]...)

				// Write complete reassembled NAL unit
				e.ffmpegStdin.Write(startCode)
				e.ffmpegStdin.Write(fuBuffer)

				// Reset buffer
				fuBuffer = nil
				fuStarted = false
			} else if fuStarted {
				// Middle fragment: accumulate data
				fuBuffer = append(fuBuffer, payload[2:]...)
			}
			// If not started, ignore middle fragments

		default:
			// Unknown type, write as-is with start code
			e.ffmpegStdin.Write(startCode)
			e.ffmpegStdin.Write(payload)
		}
	}

	// Bind sender to receiver to start receiving packets
	sender.HandleRTP(receiver)

	// Start the producer
	if err := producer.Start(); err != nil {
		log.Printf("[CVFrameExtractor] Failed to start producer for service %s: %v", e.serviceID, err)
		sender.Close()
		return
	}

	log.Printf("[CVFrameExtractor] Started H.264 → FFmpeg pipeline for service %s", e.serviceID)

	// Wait for close signal
	<-e.closeChan

	// Clean up
	sender.Close()
}

// readFramesFromFFmpeg reads JPEG frames from FFmpeg stdout
func (e *CVFrameExtractor) readFramesFromFFmpeg() {
	defer log.Printf("[CVFrameExtractor] Stopped JPEG reader for service %s", e.serviceID)
	defer close(e.frameReady)

	for {
		select {
		case <-e.closeChan:
			return
		default:
			// Read one JPEG frame
			jpeg, err := e.readJPEGFrame()
			if err != nil {
				if err != io.EOF {
					log.Printf("[CVFrameExtractor] Failed to read JPEG: %v", err)
				}
				return
			}

			// Send frame to saver (non-blocking)
			select {
			case e.frameReady <- jpeg:
			case <-e.closeChan:
				return
			default:
				// Drop frame if channel full
				log.Printf("[CVFrameExtractor] Dropped frame (channel full) for service %s", e.serviceID)
			}
		}
	}
}

// readJPEGFrame reads a single JPEG frame from FFmpeg stdout
func (e *CVFrameExtractor) readJPEGFrame() ([]byte, error) {
	// JPEG starts with 0xFFD8, ends with 0xFFD9
	var frame bytes.Buffer
	buf := make([]byte, 4096)

	// Look for JPEG start marker (0xFFD8)
	foundStart := false
	for !foundStart {
		n, err := e.ffmpegStdout.Read(buf)
		if err != nil {
			return nil, err
		}

		for i := 0; i < n-1; i++ {
			if buf[i] == 0xFF && buf[i+1] == 0xD8 {
				// Found start marker
				frame.Write(buf[i:n])
				foundStart = true
				break
			}
		}

		if !foundStart && n > 0 {
			// Keep last byte in case start marker is split
			if buf[n-1] == 0xFF {
				frame.WriteByte(0xFF)
			}
		}
	}

	// Read until end marker (0xFFD9)
	for {
		n, err := e.ffmpegStdout.Read(buf)
		if err != nil {
			return nil, err
		}

		frame.Write(buf[:n])

		// Check for end marker in what we just read
		data := frame.Bytes()
		if len(data) >= 2 && data[len(data)-2] == 0xFF && data[len(data)-1] == 0xD9 {
			return data, nil
		}
	}
}

// saveFrameLoop saves frames as they arrive
func (e *CVFrameExtractor) saveFrameLoop() {
	defer log.Printf("[CVFrameExtractor] Stopped frame saver for service %s", e.serviceID)

	for {
		select {
		case <-e.closeChan:
			return
		case jpeg, ok := <-e.frameReady:
			if !ok {
				return
			}
			e.saveFrame(jpeg)
		}
	}
}

// saveFrame saves a JPEG frame and creates a job
func (e *CVFrameExtractor) saveFrame(jpeg []byte) {
	frameUUID := uuid.New().String()
	timestamp := time.Now()

	// Save frame to disk
	framesDir := e.storageManager.GetFramesDir()
	filePath := filepath.Join(framesDir, fmt.Sprintf("%s.jpg", frameUUID))

	if err := os.WriteFile(filePath, jpeg, 0644); err != nil {
		log.Printf("[CVFrameExtractor] Failed to save frame %s: %v", frameUUID, err)
		return
	}

	// Register with storage manager
	metadata := &FrameMetadata{
		UUID:      frameUUID,
		ServiceID: e.serviceID,
		FilePath:  filePath,
		Timestamp: timestamp,
		FileSize:  int64(len(jpeg)),
		CreatedAt: timestamp,
	}

	if err := e.storageManager.RegisterFrame(metadata); err != nil {
		log.Printf("[CVFrameExtractor] Failed to register frame %s: %v", frameUUID, err)
		return
	}

	// Add frame to batch
	e.mu.Lock()
	if len(e.frameBatch) == 0 {
		e.batchStartTime = timestamp
	}
	e.frameBatch = append(e.frameBatch, frameUUID)
	shouldEmitBatch := len(e.frameBatch) >= e.batchSize
	e.mu.Unlock()

	// Emit frame event
	frameEvent := &FrameEvent{
		ServiceID: e.serviceID,
		FrameUUID: frameUUID,
	}
	e.eventBus.EmitFrameEvent(frameEvent)

	log.Printf("[CVFrameExtractor] Extracted frame %s for service %s (%d bytes)",
		frameUUID, e.serviceID, len(jpeg))

	// Emit batch event if we have enough frames
	if shouldEmitBatch {
		e.emitBatchEvent()
	}
}

// emitBatchEvent emits a frame_batch event from collected frames
func (e *CVFrameExtractor) emitBatchEvent() {
	e.mu.Lock()
	if len(e.frameBatch) == 0 {
		e.mu.Unlock()
		return
	}

	// Copy batch and reset
	batchFrames := make([]string, len(e.frameBatch))
	copy(batchFrames, e.frameBatch)
	batchStart := e.batchStartTime
	e.frameBatch = make([]string, 0, e.batchSize)
	e.mu.Unlock()

	// Calculate batch duration
	batchEnd := time.Now()
	duration := batchEnd.Sub(batchStart).Seconds()

	// Emit batch event
	batchEvent := &FrameBatchEvent{
		ServiceID: e.serviceID,
		Frames:    batchFrames,
		Metadata: map[string]interface{}{
			"duration_seconds": duration, // Temporal span of the batch
		},
	}
	e.eventBus.EmitFrameBatchEvent(batchEvent)

	log.Printf("[CVFrameExtractor] Emitted batch event with %d frames for service %s (duration=%.1fs)",
		len(batchFrames), e.serviceID, duration)
}

// Close stops frame extraction
func (e *CVFrameExtractor) Close() {
	close(e.closeChan)

	// Kill FFmpeg process if running
	e.mu.Lock()
	if e.ffmpegCmd != nil && e.ffmpegCmd.Process != nil {
		e.ffmpegCmd.Process.Kill()
		e.ffmpegCmd.Wait() // Clean up zombie process
	}
	e.mu.Unlock()
}
