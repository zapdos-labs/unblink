package webrtc

import (
	"io"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/h264"
	"github.com/AlexxIT/go2rtc/pkg/h264/annexb"
	"github.com/pion/rtp"
)

// H264Consumer streams raw H.264 in Annex-B format
// Following the MJPEG consumer pattern from go2rtc
type H264Consumer struct {
	core.Connection
	wr *core.WriteBuffer
}

// NewH264Consumer creates a new H.264 Annex-B consumer
func NewH264Consumer() *H264Consumer {
	medias := []*core.Media{
		{
			Kind:      core.KindVideo,
			Direction: core.DirectionSendonly,
			Codecs: []*core.Codec{
				{Name: core.CodecH264},
			},
		},
	}
	wr := core.NewWriteBuffer(nil)
	return &H264Consumer{
		Connection: core.Connection{
			ID:         core.NewID(),
			FormatName: "h264",
			Medias:     medias,
			Transport:  wr,
		},
		wr: wr,
	}
}

// AddTrack adds an H.264 track to the consumer
// Converts RTP packets → AVCC → Annex-B format for FFmpeg
func (c *H264Consumer) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	sender := core.NewSender(media, track.Codec)

	// Handler: converts payload to Annex-B and writes to buffer
	sender.Handler = func(packet *rtp.Packet) {
		// Convert AVCC format to Annex-B (adds 0x00000001 start codes)
		// safeClone=true creates a copy to avoid modifying original packet
		annexbData := annexb.DecodeAVCC(packet.Payload, true)
		if n, err := c.wr.Write(annexbData); err == nil {
			c.Send += n
		}
	}

	// Apply RTP depayloading if codec is RTP-based
	if track.Codec.IsRTP() {
		sender.Handler = h264.RTPDepay(track.Codec, sender.Handler)
	}

	sender.HandleRTP(track)
	c.Senders = append(c.Senders, sender)
	return nil
}

// WriteTo streams all H.264 Annex-B data to the writer
func (c *H264Consumer) WriteTo(wr io.Writer) (int64, error) {
	return c.wr.WriteTo(wr)
}

// Stop closes the consumer
func (c *H264Consumer) Stop() {
	_ = c.wr.Close()
}
