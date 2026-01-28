package webrtc

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/core"
	go2webrtc "github.com/AlexxIT/go2rtc/pkg/webrtc"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"unblink/server"
)

// Session represents an active WebRTC session
type Session struct {
	SessionID  string
	BridgeID   string
	NodeID     string
	ServiceID  string
	ServiceURL string

	webrtcConn *go2webrtc.Conn
	source     MediaSource
	bridgeConn *server.BridgeConn
	nodeConn   *server.NodeConn

	closeChan  chan struct{}
	closeOnce  sync.Once
	sessionMgr *SessionManager
}

// NewSession creates a new WebRTC session using go2rtc
func NewSession(
	ctx context.Context,
	nodeConn *server.NodeConn,
	serviceID, serviceURL string,
	sdpOffer string,
	sessionMgr *SessionManager,
) (*Session, string, error) {
	sessionID := uuid.New().String()

	log.Printf("[WebRTC Session %s] Creating session for service %s on node %s", sessionID, serviceID, nodeConn.NodeID())
	log.Printf("[WebRTC Session %s] Service URL: %s", sessionID, serviceURL)

	// Open bridge to the service
	bridgeID, dataChan, err := nodeConn.OpenBridge(ctx, serviceID, serviceURL)
	if err != nil {
		return nil, "", fmt.Errorf("open bridge: %w", err)
	}

	log.Printf("[WebRTC Session %s] Bridge %s opened", sessionID, bridgeID)

	// Create BridgeConn for the data channel
	bridgeConn := server.NewBridgeConn(nodeConn, bridgeID, dataChan)

	session := &Session{
		SessionID:  sessionID,
		BridgeID:   bridgeID,
		NodeID:     nodeConn.NodeID(),
		ServiceID:  serviceID,
		ServiceURL: serviceURL,
		bridgeConn: bridgeConn,
		nodeConn:   nodeConn,
		closeChan:  make(chan struct{}),
		sessionMgr: sessionMgr,
	}

	// Create media source based on service type (RTSP, MJPEG, etc.)
	source, err := NewMediaSource(serviceURL, bridgeID, bridgeConn)
	if err != nil {
		bridgeConn.Close()
		nodeConn.CloseBridge(ctx, bridgeID)
		return nil, "", fmt.Errorf("create source: %w", err)
	}
	session.source = source

	// Create WebRTC API
	api, err := go2webrtc.NewAPI()
	if err != nil {
		source.Close() // This closes the underlying client which closes the bridgeConn?
		// Actually source.Close closes RTSP client. RTSP client implementation might close connection.
		// Safe to call bridgeConn.Close() after just in case, or let it be.
		bridgeConn.Close()
		nodeConn.CloseBridge(ctx, bridgeID)
		return nil, "", fmt.Errorf("create WebRTC API: %w", err)
	}

	// Create peer connection with STUN server
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		source.Close()
		bridgeConn.Close()
		nodeConn.CloseBridge(ctx, bridgeID)
		return nil, "", fmt.Errorf("create peer connection: %w", err)
	}

	// Wrap with go2rtc webrtc.Conn
	wconn := go2webrtc.NewConn(pc)
	wconn.Mode = core.ModePassiveProducer
	session.webrtcConn = wconn

	// Parse the offer
	if err := wconn.SetOffer(sdpOffer); err != nil {
		session.Close()
		return nil, "", fmt.Errorf("set offer: %w", err)
	}

	// Get medias from source
	producer := source.GetProducer()
	receivers := source.GetReceivers()
	medias := producer.GetMedias()
	if len(medias) == 0 {
		session.Close()
		return nil, "", fmt.Errorf("no media streams")
	}

	// Add tracks to WebRTC connection
	webrtcMedias := wconn.GetMedias()

	// Debug: log available codecs from source and WebRTC offer
	for _, m := range medias {
		for _, c := range m.Codecs {
			log.Printf("[WebRTC Session %s] Source media: %s/%s", sessionID, m.Kind, c.Name)
		}
	}
	for _, m := range webrtcMedias {
		for _, c := range m.Codecs {
			log.Printf("[WebRTC Session %s] Browser offer: %s (%s), codec: %s", sessionID, m.Kind, m.Direction, c.Name)
		}
	}

	// Debug: log receivers
	log.Printf("[WebRTC Session %s] Available receivers: %d", sessionID, len(receivers))
	for i, r := range receivers {
		if r != nil && r.Codec != nil {
			log.Printf("[WebRTC Session %s] Receiver[%d]: %s/%d", sessionID, i, r.Codec.Name, r.Codec.ClockRate)
		} else {
			log.Printf("[WebRTC Session %s] Receiver[%d]: nil", sessionID, i)
		}
	}

	for _, sourceMedia := range medias {
		// Find matching WebRTC media by kind
		var webrtcMedia *core.Media
		for _, m := range webrtcMedias {
			if m.Kind == sourceMedia.Kind && m.Direction == core.DirectionSendonly {
				webrtcMedia = m
				break
			}
		}
		if webrtcMedia == nil {
			log.Printf("[WebRTC Session %s] No matching WebRTC media for kind=%s", sessionID, sourceMedia.Kind)
			continue
		}

		// Find matching codec
		for _, codec := range sourceMedia.Codecs {
			var webrtcCodec *core.Codec
			for _, c := range webrtcMedia.Codecs {
				if c.Name == codec.Name {
					webrtcCodec = c
					break
				}
			}
			if webrtcCodec == nil {
				log.Printf("[WebRTC Session %s] Codec %s not in WebRTC offer for %s", sessionID, codec.Name, sourceMedia.Kind)
				continue
			}

			// Add track - use receiver from RTSP, or get track from producer
			var receiver *core.Receiver
			if len(receivers) > 0 {
				// RTSP: find receiver matching this codec
				log.Printf("[WebRTC Session %s] Looking for receiver with codec=%s", sessionID, codec.Name)
				for i, r := range receivers {
					if r == nil {
						log.Printf("[WebRTC Session %s]   Receiver[%d] is nil", sessionID, i)
						continue
					}
					if r.Codec == nil {
						log.Printf("[WebRTC Session %s]   Receiver[%d].Codec is nil", sessionID, i)
						continue
					}
					log.Printf("[WebRTC Session %s]   Receiver[%d].Codec.Name=%s (looking for %s)", sessionID, i, r.Codec.Name, codec.Name)
					if r.Codec.Name == codec.Name {
						receiver = r
						log.Printf("[WebRTC Session %s]   Found matching receiver!", sessionID)
						break
					}
				}
			}

			if receiver == nil || receiver.Codec == nil {
				// No pre-created receivers: get track from producer
				log.Printf("[WebRTC Session %s] No matching receiver, calling producer.GetTrack(%s/%s)", sessionID, sourceMedia.Kind, codec.Name)
				track, err := producer.GetTrack(sourceMedia, codec)
				if err != nil {
					log.Printf("[WebRTC Session %s] GetTrack error for %s/%s: %v", sessionID, sourceMedia.Kind, codec.Name, err)
					continue
				}
				if track == nil || track.Codec == nil {
					log.Printf("[WebRTC Session %s] GetTrack returned nil for %s/%s", sessionID, sourceMedia.Kind, codec.Name)
					continue
				}
				receiver = track
				log.Printf("[WebRTC Session %s] Got track from producer: %s/%d", sessionID, receiver.Codec.Name, receiver.Codec.ClockRate)
			}

			// Final safety check
			if receiver == nil || receiver.Codec == nil {
				log.Printf("[WebRTC Session %s] ERROR: Receiver or Codec is nil for %s/%s, skipping", sessionID, sourceMedia.Kind, codec.Name)
				continue
			}

			log.Printf("[WebRTC Session %s] Calling AddTrack(media=%s, codec=%s/%d, receiver=%s/%d)",
				sessionID, webrtcMedia.Kind, webrtcCodec.Name, webrtcCodec.ClockRate, receiver.Codec.Name, receiver.Codec.ClockRate)

			// Check if sender with this codec already exists
			for _, existingSender := range wconn.Senders {
				if existingSender.Codec.Name == webrtcCodec.Name {
					log.Printf("[WebRTC Session %s] WARNING: Sender with codec %s already exists! Skipping duplicate AddTrack",
						sessionID, webrtcCodec.Name)
					continue
				}
			}

			if err := wconn.AddTrack(webrtcMedia, webrtcCodec, receiver); err != nil {
				log.Printf("[WebRTC Session %s] AddTrack error for %s/%s: %v", sessionID, sourceMedia.Kind, codec.Name, err)
				continue
			}

			log.Printf("[WebRTC Session %s] Added track: %s/%s", sessionID, sourceMedia.Kind, codec.Name)
			break // Use first codec per media
		}
	}

	// Debug: log the senders after all tracks added
	log.Printf("[WebRTC Session %s] Total senders in connection: %d", sessionID, len(wconn.Senders))
	for i, s := range wconn.Senders {
		if s == nil {
			log.Printf("[WebRTC Session %s]   Sender[%d]: nil", sessionID, i)
			continue
		}
		handlerNil := "nil"
		if s.Handler != nil {
			handlerNil = "set"
		}
		log.Printf("[WebRTC Session %s]   Sender[%d]: codec=%s, handler=%s",
			sessionID, i, s.Codec.Name, handlerNil)
	}

	// Create answer with ICE candidates
	answer, err := wconn.GetCompleteAnswer(nil, nil)
	if err != nil {
		session.Close()
		return nil, "", fmt.Errorf("create answer: %w", err)
	}

	// Add to session manager
	sessionMgr.AddSession(session)

	// Setup cleanup when WebRTC connection closes
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[WebRTC Session %s] ICE state changed: %s", sessionID, state)
		if state == webrtc.ICEConnectionStateClosed ||
			state == webrtc.ICEConnectionStateDisconnected ||
			state == webrtc.ICEConnectionStateFailed {
			session.Close()
		}
	})

	// Start media handling in goroutine - close session when producer exits
	go func() {
		log.Printf("[WebRTC Session %s] Starting %s Handle() for session %s", sessionID, sourceTypeFromURL(serviceURL), sessionID)
		if err := producer.Start(); err != nil {
			log.Printf("[WebRTC Session %s] %s Handle error for session %s: %v", sessionID, sourceTypeFromURL(serviceURL), sessionID, err)
	}
		log.Printf("[WebRTC Session %s] %s Handle() ended for session %s, closing session", sessionID, sourceTypeFromURL(serviceURL), sessionID)
		session.Close()
	}()

	log.Printf("[WebRTC Session %s] Session created successfully", sessionID)

	return session, answer, nil
}

// sourceTypeFromURL determines the source type from URL for logging
func sourceTypeFromURL(serviceURL string) string {
	if strings.HasPrefix(serviceURL, "rtsp://") || strings.HasPrefix(serviceURL, "rtsps://") {
		return "RTSP"
	}
	return "MJPEG"
}

// Close closes the session and cleans up resources
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		log.Printf("[WebRTC Session %s] Closing session", s.SessionID)

		close(s.closeChan)

		// Unregister from session manager
		if s.sessionMgr != nil {
			s.sessionMgr.RemoveSession(s.SessionID)
		}

		if s.source != nil {
			s.source.Close()
		}

		if s.bridgeConn != nil {
			s.bridgeConn.Close()
		}

		if s.BridgeID != "" && s.nodeConn != nil {
			ctx := context.Background()
			s.nodeConn.CloseBridge(ctx, s.BridgeID)
		}

		if s.webrtcConn != nil {
			s.webrtcConn.Close()
		}
	})
}
