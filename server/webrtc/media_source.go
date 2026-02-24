package webrtc

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/AlexxIT/go2rtc/pkg/core"
)

// MediaSource represents a media source that can provide a producer and receivers
type MediaSource interface {
	GetProducer() core.Producer
	GetReceivers() []*core.Receiver
	Close()
}

// SourceType represents the type of media source
type SourceType string

const (
	SourceRTSP  SourceType = "rtsp"
	SourceMJPEG SourceType = "mjpeg"
)

// NewMediaSource creates the appropriate media source based on the service URL
// It inspects the URL scheme and determines the source type (RTSP or MJPEG)
func NewMediaSource(serviceURL, bridgeID string, bridgeConn net.Conn) (MediaSource, error) {
	sourceType, err := determineSourceType(serviceURL)
	if err != nil {
		return nil, fmt.Errorf("determine source type: %w", err)
	}

	switch sourceType {
	case SourceRTSP:
		return NewRTSPSourceWithBridge(serviceURL, bridgeID, bridgeConn)
	case SourceMJPEG:
		return NewMJPEGSourceWithBridge(serviceURL, bridgeID, bridgeConn)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// determineSourceType inspects the service URL to determine the media source type
func determineSourceType(serviceURL string) (SourceType, error) {
	parsed, err := url.Parse(serviceURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "rtsp", "rtsps":
		return SourceRTSP, nil
	case "http", "https":
		// Check if this is an MJPEG stream by looking at the path
		// Common MJPEG indicators: .mjpg, .mjpeg, /video, /stream, etc.
		path := strings.ToLower(parsed.Path)
		if strings.Contains(path, ".mjpg") ||
			strings.Contains(path, ".mjpeg") ||
			strings.Contains(path, "/video") ||
			strings.Contains(path, "/stream") ||
			strings.Contains(path, "mjpeg") ||
			strings.Contains(path, "mjpg") {
			return SourceMJPEG, nil
		}
		// Default to HTTP as MJPEG if we can't determine otherwise
		// (many IP cameras serve MJPEG over HTTP without obvious indicators)
		return SourceMJPEG, nil
	default:
		return "", fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
}
