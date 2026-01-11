package relay

import (
	"fmt"
	"log"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/core"
	go2webrtc "github.com/AlexxIT/go2rtc/pkg/webrtc"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/unblink/unblink/node"
	"github.com/unblink/unblink/relay/sources"
)

// WebRTCSession manages a single WebRTC streaming session
type WebRTCSession struct {
	id          string
	service     node.Service
	relay       *Relay
	webrtcConn  *go2webrtc.Conn
	source      sources.Source
	bridgeProxy *BridgeTCPProxy
	closeOnce   sync.Once
	closeChan   chan struct{}
	onClose     func() // Called when session closes to remove from manager
}

// WebRTCSessionManager manages all active WebRTC sessions
type WebRTCSessionManager struct {
	sessions map[string]*WebRTCSession
	mu       sync.RWMutex
}

// NewWebRTCSessionManager creates a new session manager
func NewWebRTCSessionManager() *WebRTCSessionManager {
	return &WebRTCSessionManager{
		sessions: make(map[string]*WebRTCSession),
	}
}

// NewSession creates a new WebRTC session from an offer and service
func (m *WebRTCSessionManager) NewSession(offerSDP string, service node.Service, relay *Relay) (string, string, error) {
	// Find node that provides the service
	nodeID := relay.Services().GetNodeID(service.ID)
	if nodeID == "" {
		return "", "", fmt.Errorf("service %s not found on any node", service.ID)
	}

	nc := relay.GetNode(nodeID)
	if nc == nil {
		return "", "", fmt.Errorf("node %s not connected", nodeID)
	}

	// Open bridge through node to the service
	bridgeID, err := nc.OpenBridge(service)
	if err != nil {
		return "", "", fmt.Errorf("open bridge: %w", err)
	}

	log.Printf("[WebRTC] Opened bridge %s to service %s", bridgeID, service.ID)

	// Create TCP proxy for the bridge
	bridgeProxy, err := NewBridgeTCPProxy(nc, bridgeID, service)
	if err != nil {
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("create bridge proxy: %w", err)
	}

	proxyAddr := bridgeProxy.Addr()
	log.Printf("[WebRTC] Bridge proxy listening on: %s", proxyAddr)

	// Create source (RTSP or MJPEG based on service type)
	source, err := sources.New(service, proxyAddr)
	if err != nil {
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("create source: %w", err)
	}

	// Create WebRTC API
	api, err := go2webrtc.NewAPI()
	if err != nil {
		source.Close()
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("create WebRTC API: %w", err)
	}

	// Create peer connection
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		source.Close()
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("create peer connection: %w", err)
	}

	// Wrap with go2rtc webrtc.Conn
	wconn := go2webrtc.NewConn(pc)
	wconn.Mode = core.ModePassiveProducer

	// Parse the offer
	if err := wconn.SetOffer(offerSDP); err != nil {
		pc.Close()
		source.Close()
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("set offer: %w", err)
	}

	// Get medias from source
	producer := source.GetProducer()
	receivers := source.GetReceivers()
	medias := producer.GetMedias()
	if len(medias) == 0 {
		pc.Close()
		source.Close()
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("no media streams")
	}

	// Add tracks to WebRTC connection
	webrtcMedias := wconn.GetMedias()

	// Debug: log available codecs from source and WebRTC offer
	for _, m := range medias {
		var codecNames []string
		for _, c := range m.Codecs {
			codecNames = append(codecNames, fmt.Sprintf("%s/%dHz", c.Name, c.ClockRate))
		}
		log.Printf("[WebRTC] Source media: %s, codecs: %v", m.Kind, codecNames)
	}
	for _, m := range webrtcMedias {
		var codecNames []string
		for _, c := range m.Codecs {
			codecNames = append(codecNames, fmt.Sprintf("%s/%dHz", c.Name, c.ClockRate))
		}
		log.Printf("[WebRTC] Browser offer: %s (%s), codecs: %v", m.Kind, m.Direction, codecNames)
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
			log.Printf("[WebRTC] No matching WebRTC media for kind=%s", sourceMedia.Kind)
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
				log.Printf("[WebRTC] Codec %s not in WebRTC offer for %s", codec.Name, sourceMedia.Kind)
				continue
			}

			// Add track - use receiver from RTSP, or get track from MJPEG producer
			var receiver *core.Receiver
			if len(receivers) > 0 {
				// RTSP: find receiver matching this codec
				for _, r := range receivers {
					if r != nil && r.Codec.Name == codec.Name {
						receiver = r
						break
					}
				}
			}

			if receiver == nil {
				// No pre-created receivers: get track from producer
				track, err := producer.GetTrack(sourceMedia, codec)
				if err != nil {
					log.Printf("[WebRTC] GetTrack error for %s/%s: %v", sourceMedia.Kind, codec.Name, err)
					continue
				}
				receiver = track
			}

			if err := wconn.AddTrack(webrtcMedia, webrtcCodec, receiver); err != nil {
				log.Printf("[WebRTC] AddTrack error for %s/%s: %v", sourceMedia.Kind, codec.Name, err)
				continue
			}

			log.Printf("[WebRTC] Added track: %s/%s", sourceMedia.Kind, codec.Name)
			break // Use first codec per media
		}
	}

	// Create answer with ICE candidates
	answerSDP, err := wconn.GetCompleteAnswer(nil, nil)
	if err != nil {
		pc.Close()
		source.Close()
		bridgeProxy.Close()
		nc.CloseBridge(bridgeID)
		return "", "", fmt.Errorf("create answer: %w", err)
	}

	// Create session first so we can reference it in the goroutine
	sessionID := uuid.New().String()
	closeChan := make(chan struct{})
	session := &WebRTCSession{
		id:          sessionID,
		service:     service,
		relay:       relay,
		webrtcConn:  wconn,
		source:      source,
		bridgeProxy: bridgeProxy,
		closeChan:   closeChan,
	}

	// Set onClose to remove session from manager
	session.onClose = func() {
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()
		log.Printf("[WebRTC] Removed session %s from manager", sessionID)
	}

	// Store session
	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Setup cleanup when WebRTC connection closes
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[WebRTC] ICE state changed: %s", state)
		if state == webrtc.ICEConnectionStateClosed ||
			state == webrtc.ICEConnectionStateDisconnected ||
			state == webrtc.ICEConnectionStateFailed {
			session.Close()
		}
	})

	// Start media handling in goroutine - close session when producer exits
	go func() {
		log.Printf("[WebRTC] Starting %s Handle() for session %s", service.Type, sessionID)
		if err := producer.Start(); err != nil {
			log.Printf("[WebRTC] %s Handle error for session %s: %v", service.Type, sessionID, err)
		}
		log.Printf("[WebRTC] %s Handle() ended for session %s, closing session", service.Type, sessionID)
		session.Close()
	}()

	log.Printf("[WebRTC] Created session %s for service %s (type=%s)", sessionID, service.ID, service.Type)

	return sessionID, answerSDP, nil
}

// Close closes a session
func (s *WebRTCSession) Close() {
	s.closeOnce.Do(func() {
		log.Printf("[WebRTC] Closing session %s", s.id)

		close(s.closeChan)

		if s.bridgeProxy != nil {
			s.bridgeProxy.Close()
		}

		if s.source != nil {
			s.source.Close()
		}

		if s.webrtcConn != nil {
			s.webrtcConn.Close()
		}

		// Remove from session manager
		if s.onClose != nil {
			s.onClose()
		}
	})
}

// GetSession retrieves a session by ID
func (m *WebRTCSessionManager) GetSession(id string) *WebRTCSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// CloseSession closes and removes a session
func (m *WebRTCSessionManager) CloseSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[id]; exists {
		session.Close()
		delete(m.sessions, id)
	}
}
