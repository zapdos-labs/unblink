package webrtc

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"

	"github.com/zapdos-labs/unblink/database"
)

const partialClipSuffix = ".part"

var clipSegmentOpenPattern = regexp.MustCompile(`Opening '([^']+)' for writing`)

type clipSegmentInfo struct {
	path      string
	startTime time.Time
}

// ClipRecorder records normalized H.264 into independently playable MP4 clips.
type ClipRecorder struct {
	serviceID       string
	baseDir         string
	segmentDuration time.Duration
	onSaved         func(serviceID, clipPath string, startTime, endTime time.Time, fileSize int64, metadata *database.ClipMetadata)

	ffmpegCmd    *exec.Cmd
	ffmpegStdin  io.WriteCloser
	ffmpegStderr io.ReadCloser

	closeOnce sync.Once
	wg        sync.WaitGroup

	mu             sync.Mutex
	currentSegment *clipSegmentInfo
	savedPaths     map[string]struct{}
}

// NewClipRecorder creates a recorder for 15-minute MP4 segments.
func NewClipRecorder(
	serviceID string,
	baseDir string,
	segmentDuration time.Duration,
	onSaved func(serviceID, clipPath string, startTime, endTime time.Time, fileSize int64, metadata *database.ClipMetadata),
) *ClipRecorder {
	return &ClipRecorder{
		serviceID:       serviceID,
		baseDir:         baseDir,
		segmentDuration: segmentDuration,
		onSaved:         onSaved,
		savedPaths:      make(map[string]struct{}),
	}
}

// Start begins recording clips from the media source.
func (r *ClipRecorder) Start(mediaSource MediaSource) error {
	if strings.TrimSpace(r.baseDir) == "" {
		log.Printf("[ClipRecorder] Skipping clip recording for service %s - no baseDir configured", r.serviceID)
		return nil
	}

	producer := mediaSource.GetProducer()
	if producer == nil {
		return fmt.Errorf("media source has no producer")
	}

	serviceClipsDir := filepath.Join(r.baseDir, r.serviceID)
	if err := os.MkdirAll(serviceClipsDir, 0o755); err != nil {
		return fmt.Errorf("create clips directory: %w", err)
	}

	if err := r.startFFmpeg(serviceClipsDir); err != nil {
		return err
	}

	r.wg.Add(2)
	go r.consumeH264ToFFmpeg(producer)
	go r.monitorFFmpegStderr()

	log.Printf("[ClipRecorder] Started clip recording for service %s into %s", r.serviceID, serviceClipsDir)
	return nil
}

func (r *ClipRecorder) startFFmpeg(serviceClipsDir string) error {
	outputPattern := filepath.Join(serviceClipsDir, "clip-%Y%m%dT%H%M%SZ"+partialClipSuffix)

	cmd := exec.Command(
		"ffmpeg",
		"-loglevel", "info",
		"-fflags", "+genpts",
		"-use_wallclock_as_timestamps", "1",
		"-f", "h264",
		"-i", "pipe:0",
		"-an",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-force_key_frames", "expr:gte(t,n_forced*1)",
		"-sc_threshold", "0",
		"-pix_fmt", "yuv420p",
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%.0f", r.segmentDuration.Seconds()),
		"-segment_atclocktime", "1",
		"-reset_timestamps", "1",
		"-segment_format", "mp4",
		"-segment_format_options", "movflags=+faststart",
		"-strftime", "1",
		outputPattern,
	)
	cmd.Env = append(os.Environ(), "TZ=UTC")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdin pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	r.ffmpegCmd = cmd
	r.ffmpegStdin = stdin
	r.ffmpegStderr = stderr
	return nil
}

func (r *ClipRecorder) consumeH264ToFFmpeg(producer core.Producer) {
	defer r.wg.Done()
	defer log.Printf("[ClipRecorder] Stopped H.264 writer for service %s", r.serviceID)

	h264Consumer := NewH264Consumer()
	defer h264Consumer.Stop()

	videoMedia, h264Codec, receiver, err := getH264Track(producer)
	if err != nil {
		log.Printf("[ClipRecorder] Failed to get H.264 track for service %s: %v", r.serviceID, err)
		return
	}

	if err := h264Consumer.AddTrack(videoMedia, h264Codec, receiver); err != nil {
		log.Printf("[ClipRecorder] Failed to add H.264 track for service %s: %v", r.serviceID, err)
		return
	}

	if _, err := h264Consumer.WriteTo(r.ffmpegStdin); err != nil && !isClosedPipeError(err) {
		log.Printf("[ClipRecorder] H.264 writer exited for service %s: %v", r.serviceID, err)
	}
}

func (r *ClipRecorder) monitorFFmpegStderr() {
	defer r.wg.Done()
	defer log.Printf("[ClipRecorder] Stopped FFmpeg monitor for service %s", r.serviceID)

	scanner := bufio.NewScanner(r.ffmpegStderr)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[ClipRecorder] FFmpeg %s", line)

		match := clipSegmentOpenPattern.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}

		clipPath := match[1]
		startTime, err := parseClipStartTime(clipPath)
		if err != nil {
			log.Printf("[ClipRecorder] Failed to parse clip start time for %s: %v", clipPath, err)
			continue
		}

		r.handleSegmentOpen(clipPath, startTime)
	}

	if err := scanner.Err(); err != nil && !isClosedPipeError(err) {
		log.Printf("[ClipRecorder] FFmpeg stderr scanner error for service %s: %v", r.serviceID, err)
	}
}

func (r *ClipRecorder) handleSegmentOpen(clipPath string, startTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentSegment != nil {
		r.saveSegmentLocked(r.currentSegment.path, r.currentSegment.startTime, startTime)
	}

	r.currentSegment = &clipSegmentInfo{
		path:      clipPath,
		startTime: startTime,
	}
}

// Close stops the recorder and persists the final segment if it exists.
func (r *ClipRecorder) Close() {
	r.closeOnce.Do(func() {
		log.Printf("[ClipRecorder] Closing recorder for service %s", r.serviceID)

		if r.ffmpegStdin != nil {
			_ = r.ffmpegStdin.Close()
		}

		waitDone := make(chan error, 1)
		go func() {
			if r.ffmpegCmd == nil {
				waitDone <- nil
				return
			}
			waitDone <- r.ffmpegCmd.Wait()
		}()

		select {
		case err := <-waitDone:
			if err != nil {
				log.Printf("[ClipRecorder] FFmpeg exited for service %s: %v", r.serviceID, err)
			}
		case <-time.After(5 * time.Second):
			if r.ffmpegCmd != nil && r.ffmpegCmd.Process != nil {
				_ = r.ffmpegCmd.Process.Kill()
				<-waitDone
			}
		}

		done := make(chan struct{})
		go func() {
			r.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("[ClipRecorder] Timeout waiting for recorder goroutines for service %s", r.serviceID)
		}

		r.flushFinalSegment()
	})
}

func (r *ClipRecorder) flushFinalSegment() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentSegment == nil {
		return
	}

	endTime := r.estimateSegmentEnd(r.currentSegment.path, r.currentSegment.startTime)
	r.saveSegmentLocked(r.currentSegment.path, r.currentSegment.startTime, endTime)
	r.currentSegment = nil
}

func (r *ClipRecorder) estimateSegmentEnd(clipPath string, startTime time.Time) time.Time {
	info, err := os.Stat(clipPath)
	if err != nil {
		return startTime.Add(r.segmentDuration)
	}

	endTime := info.ModTime().UTC()
	maxEnd := startTime.Add(r.segmentDuration)
	if endTime.After(maxEnd) {
		return maxEnd
	}
	if endTime.Before(startTime) {
		return maxEnd
	}
	return endTime
}

func (r *ClipRecorder) saveSegmentLocked(clipPath string, startTime, endTime time.Time) {
	finalPath := finalizedClipPath(clipPath)
	if _, exists := r.savedPaths[finalPath]; exists {
		return
	}
	if !endTime.After(startTime) {
		return
	}

	info, err := os.Stat(clipPath)
	if err != nil {
		log.Printf("[ClipRecorder] Failed to stat clip %s: %v", clipPath, err)
		return
	}
	if info.Size() == 0 {
		return
	}
	if clipPath != finalPath {
		if err := os.Rename(clipPath, finalPath); err != nil {
			log.Printf("[ClipRecorder] Failed to finalize clip %s: %v", clipPath, err)
			return
		}

		info, err = os.Stat(finalPath)
		if err != nil {
			log.Printf("[ClipRecorder] Failed to stat finalized clip %s: %v", finalPath, err)
			return
		}
	}

	metadata := &database.ClipMetadata{
		EndTime:                endTime.UTC().Format(time.RFC3339),
		DurationSeconds:        int64(endTime.Sub(startTime).Seconds()),
		Container:              "mp4",
		VideoCodec:             "h264",
		SourceKind:             "normalized_h264",
		SegmentDurationSeconds: int64(r.segmentDuration.Seconds()),
	}

	if r.onSaved != nil {
		r.onSaved(r.serviceID, finalPath, startTime.UTC(), endTime.UTC(), info.Size(), metadata)
	}
	r.savedPaths[finalPath] = struct{}{}
}

func parseClipStartTime(clipPath string) (time.Time, error) {
	name := filepath.Base(clipPath)
	for {
		ext := filepath.Ext(name)
		if ext == "" {
			break
		}
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimPrefix(name, "clip-")
	return time.Parse("20060102T150405Z", name)
}

func finalizedClipPath(clipPath string) string {
	if strings.HasSuffix(clipPath, partialClipSuffix) {
		return strings.TrimSuffix(clipPath, partialClipSuffix) + ".mp4"
	}
	return clipPath
}

func getH264Track(producer core.Producer) (*core.Media, *core.Codec, *core.Receiver, error) {
	var videoMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindVideo {
			videoMedia = media
			break
		}
	}
	if videoMedia == nil {
		return nil, nil, nil, fmt.Errorf("no video media found")
	}

	var h264Codec *core.Codec
	for _, codec := range videoMedia.Codecs {
		if codec.Name == core.CodecH264 {
			h264Codec = codec
			break
		}
	}
	if h264Codec == nil {
		return nil, nil, nil, fmt.Errorf("no H.264 codec found")
	}

	receiver, err := producer.GetTrack(videoMedia, h264Codec)
	if err != nil {
		return nil, nil, nil, err
	}

	return videoMedia, h264Codec, receiver, nil
}

func isClosedPipeError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "closed pipe")
}
