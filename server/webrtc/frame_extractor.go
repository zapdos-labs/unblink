package webrtc

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

// Frame represents a single extracted JPEG frame
type Frame struct {
	Data      []byte    // JPEG bytes
	Timestamp time.Time // When the frame was extracted
	ServiceID string    // Service identifier (e.g., camera name)
	Sequence  int64     // Monotonically increasing sequence number
}

// FrameExtractor extracts JPEG frames from H.264 streams using FFmpeg
type FrameExtractor struct {
	serviceID string
	interval  time.Duration
	onFrame   func(*Frame) // Callback when frame is ready
	closeChan chan struct{}
	closeOnce sync.Once

	// FFmpeg pipeline
	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStdout io.ReadCloser

	// Frame sequencing
	sequence int64
	mu       sync.Mutex
}

// NewFrameExtractor creates a new frame extractor
func NewFrameExtractor(serviceID string, interval time.Duration, onFrame func(*Frame)) *FrameExtractor {
	return &FrameExtractor{
		serviceID: serviceID,
		interval:  interval,
		onFrame:   onFrame,
		closeChan: make(chan struct{}),
	}
}

// Start begins extracting frames from the media source
func (e *FrameExtractor) Start(mediaSource MediaSource) error {
	log.Printf("[FrameExtractor] Starting frame extraction for service %s (interval=%v)", e.serviceID, e.interval)

	// Start FFmpeg process
	if err := e.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Get producer from media source
	producer := mediaSource.GetProducer()
	if producer == nil {
		return fmt.Errorf("media source has no producer")
	}

	// Start H.264 packet consumer that pipes to FFmpeg
	go e.consumeH264ToFFmpeg(producer)

	// Start JPEG frame reader from FFmpeg
	go e.readFramesFromFFmpeg()

	return nil
}

// startFFmpeg starts the FFmpeg process for H.264 to JPEG conversion
func (e *FrameExtractor) startFFmpeg() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Calculate fps from interval (e.g., 5s interval = 1/5 fps = 0.2 fps)
	fps := 1.0 / e.interval.Seconds()

	// FFmpeg command: H.264 stdin → JPEG frames at specified fps
	// Using raw H.264 Annex-B format from H264Consumer
	e.ffmpegCmd = exec.Command(
		"ffmpeg",
		"-loglevel", "error", // Only log errors
		"-f", "h264", // Input format (raw H.264 Annex-B)
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

	log.Printf("[FrameExtractor] Started FFmpeg for service %s (fps=%.3f)", e.serviceID, fps)
	return nil
}

// consumeH264ToFFmpeg reads H.264 packets and pipes them directly to FFmpeg
func (e *FrameExtractor) consumeH264ToFFmpeg(producer core.Producer) {
	defer log.Printf("[FrameExtractor] Stopped H.264 consumer for service %s", e.serviceID)

	// Create H.264 consumer
	h264Consumer := NewH264Consumer()
	defer h264Consumer.Stop()

	// Find H.264 video track
	var videoMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindVideo {
			videoMedia = media
			break
		}
	}
	if videoMedia == nil {
		log.Printf("[FrameExtractor] No video media found for service %s", e.serviceID)
		return
	}

	var h264Codec *core.Codec
	for _, codec := range videoMedia.Codecs {
		if codec.Name == core.CodecH264 {
			h264Codec = codec
			break
		}
	}
	if h264Codec == nil {
		log.Printf("[FrameExtractor] No H.264 codec found for service %s", e.serviceID)
		return
	}

	// Get track from producer
	receiver, err := producer.GetTrack(videoMedia, h264Codec)
	if err != nil {
		log.Printf("[FrameExtractor] Failed to get H.264 track: %v", err)
		return
	}

	// Add track to consumer
	if err := h264Consumer.AddTrack(videoMedia, h264Codec, receiver); err != nil {
		log.Printf("[FrameExtractor] Failed to add track: %v", err)
		return
	}

	log.Printf("[FrameExtractor] Starting to write H.264 to FFmpeg for service %s", e.serviceID)

	// WriteTo blocks until error
	written, err := h264Consumer.WriteTo(e.ffmpegStdin)
	if err != nil {
		log.Printf("[FrameExtractor] H.264 writer finished: written=%d err=%v", written, err)
	} else {
		log.Printf("[FrameExtractor] H.264 writer finished: written=%d", written)
	}
}

// readFramesFromFFmpeg reads JPEG frames from FFmpeg stdout
func (e *FrameExtractor) readFramesFromFFmpeg() {
	defer log.Printf("[FrameExtractor] Stopped JPEG reader for service %s", e.serviceID)

	// JPEG SOI (Start of Image) marker
	soi := []byte{0xFF, 0xD8}
	// JPEG EOI (End of Image) marker
	eoi := []byte{0xFF, 0xD9}

	var frameBuffer bytes.Buffer
	buf := make([]byte, 4096)
	inFrame := false

	log.Printf("[FrameExtractor] Starting to read JPEG frames from FFmpeg for service %s", e.serviceID)

	for {
		select {
		case <-e.closeChan:
			log.Printf("[FrameExtractor] JPEG reader closing")
			return
		default:
		}

		n, err := e.ffmpegStdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[FrameExtractor] Error reading from FFmpeg output pipe: %v", err)
			} else {
				log.Printf("[FrameExtractor] FFmpeg stdout closed")
			}
			return
		}

		for i := 0; i < n; i++ {
			b := buf[i]
			frameBuffer.WriteByte(b)

			// Check for SOI marker
			if frameBuffer.Len() >= 2 {
				last2 := frameBuffer.Bytes()[frameBuffer.Len()-2:]
				if bytes.Equal(last2, soi) && !inFrame {
					inFrame = true
					frameBuffer.Reset()
					frameBuffer.Write(soi)
				}
			}

			// Check for EOI marker
			if inFrame && frameBuffer.Len() >= 2 {
				last2 := frameBuffer.Bytes()[frameBuffer.Len()-2:]
				if bytes.Equal(last2, eoi) {
					// Complete JPEG frame
					frameData := make([]byte, frameBuffer.Len())
					copy(frameData, frameBuffer.Bytes())

					// Increment sequence
					e.mu.Lock()
					e.sequence++
					seq := e.sequence
					e.mu.Unlock()

					// Create frame
					frame := &Frame{
						Data:      frameData,
						Timestamp: time.Now(),
						ServiceID: e.serviceID,
						Sequence:  seq,
					}

					log.Printf("[FrameExtractor] Extracted JPEG frame %d (%d bytes)", seq, len(frameData))

					// Call callback
					if e.onFrame != nil {
						e.onFrame(frame)
					}

					// Reset for next frame
					frameBuffer.Reset()
					inFrame = false
				}
			}
		}
	}
}

// Close stops the frame extractor
func (e *FrameExtractor) Close() {
	e.closeOnce.Do(func() {
		log.Printf("[FrameExtractor] Closing frame extractor for service %s", e.serviceID)
		close(e.closeChan)

		// Wait a bit for goroutines to finish
		time.Sleep(100 * time.Millisecond)

		// Kill FFmpeg if still running
		if e.ffmpegCmd != nil && e.ffmpegCmd.Process != nil {
			e.ffmpegCmd.Process.Kill()
			e.ffmpegCmd.Wait()
		}
	})
}
