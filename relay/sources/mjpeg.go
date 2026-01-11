package sources

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/magic"
	"github.com/unblink/unblink/node"
)

// MJPEGSource handles MJPEG streams by transcoding to H.264 via FFmpeg
type MJPEGSource struct {
	producer  core.Producer
	cmd       *exec.Cmd
	receivers []*core.Receiver
}

// NewMJPEG creates a new MJPEG source that transcodes to H.264 using FFmpeg
func NewMJPEG(service node.Service, proxyAddr string) (*MJPEGSource, error) {
	// Build the input URL pointing to the bridge proxy
	inputURL := fmt.Sprintf("http://%s%s", proxyAddr, service.Path)

	// Build FFmpeg command to transcode MJPEG -> H.264
	cmd := exec.Command("ffmpeg",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-i", inputURL,
		"-c:v", "libx264",
		"-preset", "superfast",
		"-tune", "zerolatency",
		"-g", "50",
		"-profile:v", "high",
		"-level:v", "4.1",
		"-pix_fmt:v", "yuv420p",
		"-f", "mpegts",
		"-",
	)

	// Capture stdout for reading H.264 bitstream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("FFmpeg stdout pipe: %w", err)
	}

	// Capture stderr for logging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("FFmpeg stderr pipe: %w", err)
	}

	log.Printf("[MJPEG] Starting FFmpeg for MJPEG->H.264 transcoding: %v", cmd.Args)

	// Start FFmpeg process
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("FFmpeg start: %w", err)
	}

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[FFmpeg] %s", scanner.Text())
		}
	}()

	// Create a buffered reader for the H.264 stream
	rd := bufio.NewReaderSize(stdout, core.BufferSize)

	// Use go2rtc's magic.Open to parse the H.264 bitstream
	prod, err := magic.Open(rd)
	if err != nil {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("magic.Open H.264: %w", err)
	}

	log.Printf("[MJPEG] FFmpeg H.264 transcoding started")

	// Get medias from producer
	medias := prod.GetMedias()
	if len(medias) == 0 {
		_ = prod.Stop()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("no media from FFmpeg")
	}

	// Get receivers for H.264 codec
	var receivers []*core.Receiver
	for _, media := range medias {
		for _, codec := range media.Codecs {
			if codec.Name != core.CodecH264 {
				continue
			}
			receiver, err := prod.GetTrack(media, codec)
			if err != nil {
				log.Printf("[MJPEG] Failed to get track: %v", err)
				continue
			}
			receivers = append(receivers, receiver)
			log.Printf("[MJPEG] Added FFmpeg H.264 track: %s/%s", media.Kind, codec.Name)
			break
		}
	}

	if len(receivers) == 0 {
		_ = prod.Stop()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("no H.264 codec from FFmpeg")
	}

	return &MJPEGSource{
		producer:  prod,
		cmd:       cmd,
		receivers: receivers,
	}, nil
}

// GetProducer returns the H.264 producer from FFmpeg transcoding
func (s *MJPEGSource) GetProducer() core.Producer {
	return s.producer
}

// GetReceivers returns the receivers for the transcoded H.264 track
func (s *MJPEGSource) GetReceivers() []*core.Receiver {
	return s.receivers
}

// Close stops the FFmpeg process and producer
func (s *MJPEGSource) Close() {
	if s.producer != nil {
		_ = s.producer.Stop()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait() // Clean up zombie process
	}
}
