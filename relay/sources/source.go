package sources

import (
	"fmt"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/unblink/unblink/node"
)

// Source represents a media source that can provide a producer and receivers
type Source interface {
	GetProducer() core.Producer
	GetReceivers() []*core.Receiver
	Close()
}

// New creates the appropriate source for the given service type
func New(service node.Service, proxyAddr string) (Source, error) {
	switch service.Type {
	case "rtsp":
		return NewRTSP(service, proxyAddr)
	case "mjpeg":
		return NewMJPEG(service, proxyAddr)
	default:
		return nil, fmt.Errorf("unsupported service type: %s", service.Type)
	}
}
