package webrtc

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"unblink/server/internal/rtsp"
)

// RTSPSource handles RTSP stream sources
type RTSPSource struct {
	client    *rtsp.Conn
	receivers []*core.Receiver
}

// NewRTSPSourceWithBridge creates a new RTSP source using a direct bridge connection
func NewRTSPSourceWithBridge(serviceURL, bridgeID string, bridgeConn net.Conn) (*RTSPSource, error) {

	log.Printf("[RTSP] Starting RTSP source for bridge %s", bridgeID)
	log.Printf("[RTSP] RTSP URL: %s", serviceURL)

	// Create RTSP client
	client := rtsp.NewClient(serviceURL)

	// Inject custom bridge connection
	log.Printf("[RTSP] Injecting custom bridge connection")
	client.SetConn(bridgeConn)

	log.Printf("[RTSP] Dialing RTSP server...")

	// Connect to RTSP server
	if err := client.Dial(); err != nil {
		log.Printf("[RTSP] Dial failed: %v", err)
		return nil, fmt.Errorf("RTSP dial: %w", err)
	}

	log.Printf("[RTSP] Dial successful, sending DESCRIBE...")

	// Describe to get media info
	if err := client.Describe(); err != nil {
		client.Close()
		log.Printf("[RTSP] Describe failed: %v", err)
		return nil, fmt.Errorf("RTSP describe: %w", err)
	}

	// Get medias from RTSP
	rtspMedias := client.GetMedias()
	if len(rtspMedias) == 0 {
		client.Close()
		return nil, fmt.Errorf("no media streams in RTSP")
	}

	// Log available media
	for _, media := range rtspMedias {
		for _, codec := range media.Codecs {
			log.Printf("[RTSP] Available media: %s/%s", media.Kind, codec.Name)
		}
	}

	// Get receivers for all media/codecs (first codec per media)
	var receivers []*core.Receiver
	for _, media := range rtspMedias {
		for _, codec := range media.Codecs {
			receiver, err := client.GetTrack(media, codec)
			if err != nil {
				log.Printf("[RTSP] GetTrack error for %s/%s: %v", media.Kind, codec.Name, err)
				continue
			}
			receivers = append(receivers, receiver)
			log.Printf("[RTSP] Added track: %s/%s", media.Kind, codec.Name)
			break // Use first codec per media
		}
	}

	if len(receivers) == 0 {
		client.Close()
		return nil, fmt.Errorf("no codecs found")
	}

	// Start RTSP playback
	log.Printf("[RTSP] Starting PLAY...")
	if err := client.Play(); err != nil {
		client.Close()
		log.Printf("[RTSP] Play failed: %v", err)
		return nil, fmt.Errorf("RTSP play: %w", err)
	}

	log.Printf("[RTSP] Connected successfully, %d track(s)", len(receivers))

	source := &RTSPSource{
		client:    client,
		receivers: receivers,
	}

	// Start a goroutine to monitor if we're receiving data
	go func() {
		time.Sleep(2 * time.Second)
		// Check if RTSP client is still connected
		if source.client != nil {
			medias := source.client.GetMedias()
			log.Printf("[RTSP] Client still connected, %d media(s)", len(medias))
		}
	}()

	return source, nil
}

// GetProducer implements MediaSource
func (s *RTSPSource) GetProducer() core.Producer {
	return s.client
}

// GetReceivers implements MediaSource
func (s *RTSPSource) GetReceivers() []*core.Receiver {
	return s.receivers
}

// Close implements MediaSource
func (s *RTSPSource) Close() {
	if s.client != nil {
		s.client.Close()
	}
}
