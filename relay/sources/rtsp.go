package sources

import (
	"fmt"
	"log"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/rtsp"
	"github.com/zapdos-labs/unblink/node"
)

// RTSPSource handles RTSP stream sources
type RTSPSource struct {
	client    *rtsp.Conn
	receivers []*core.Receiver
}

// NewRTSP creates a new RTSP source from the given service and proxy address
func NewRTSP(service node.Service, proxyAddr string) (*RTSPSource, error) {
	// Build RTSP URL with auth if present
	rtspURL := "rtsp://"
	if service.Auth != nil {
		rtspURL += fmt.Sprintf("%s:%s@", service.Auth.Username, service.Auth.Password)
	}
	rtspURL += proxyAddr + service.Path

	// Create RTSP client
	client := rtsp.NewClient(rtspURL)
	client.Transport = "tcp"

	// Connect to RTSP server
	if err := client.Dial(); err != nil {
		return nil, fmt.Errorf("RTSP dial: %w", err)
	}

	// Describe to get media info
	if err := client.Describe(); err != nil {
		client.Close()
		return nil, fmt.Errorf("RTSP describe: %w", err)
	}

	// Get medias from RTSP
	rtspMedias := client.GetMedias()
	if len(rtspMedias) == 0 {
		client.Close()
		return nil, fmt.Errorf("no media streams in RTSP")
	}

	// Get receivers for all media/codecs (first codec per media)
	var receivers []*core.Receiver
	for _, media := range rtspMedias {
		for _, codec := range media.Codecs {
			receiver, err := client.GetTrack(media, codec)
			if err != nil {
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
	if err := client.Play(); err != nil {
		client.Close()
		return nil, fmt.Errorf("RTSP play: %w", err)
	}

	return &RTSPSource{
		client:    client,
		receivers: receivers,
	}, nil
}

// GetProducer returns the RTSP client as a producer
func (s *RTSPSource) GetProducer() core.Producer {
	return s.client
}

// GetReceivers returns the receivers for the RTSP tracks
func (s *RTSPSource) GetReceivers() []*core.Receiver {
	return s.receivers
}

// Close stops the RTSP client
func (s *RTSPSource) Close() {
	if s.client != nil {
		s.client.Stop()
	}
}
