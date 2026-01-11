package relay_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/webrtc"
	"github.com/google/uuid"
	pionwebrtc "github.com/pion/webrtc/v4"
	"github.com/unblink/unblink/node"
	"github.com/unblink/unblink/relay"
)

// startHTTPAPIListener starts the HTTP API on a random port and returns the listener
func startHTTPAPIListener(r *relay.Relay, addr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	sessionManager := relay.NewWebRTCSessionManager()

	// Node-specific endpoints: /node/{nodeId}/services and /node/{nodeId}/offer
	mux.HandleFunc("/node/", func(w http.ResponseWriter, req *http.Request) {
		// Parse: /node/{nodeId}/{endpoint}
		path := strings.TrimPrefix(req.URL.Path, "/node/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		nodeID := parts[0]
		endpoint := parts[1]

		switch endpoint {
		case "services":
			services := r.Services().ListByNode(nodeID)
			nodeExists := r.GetNode(nodeID) != nil || len(services) > 0
			if !nodeExists {
				http.Error(w, "Node not found: "+nodeID, http.StatusNotFound)
				return
			}
			type ServiceInfo struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				NodeID string `json:"node_id"`
				Addr   string `json:"addr"`
				Port   int    `json:"port"`
				Path   string `json:"path"`
			}
			result := make([]ServiceInfo, len(services))
			for i, s := range services {
				result[i] = ServiceInfo{
					ID:     s.Service.ID,
					Type:   s.Service.Type,
					NodeID: s.NodeID,
					Addr:   s.Service.Addr,
					Port:   s.Service.Port,
					Path:   s.Service.Path,
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)

		case "offer":
			var offerReq struct {
				SDP       string `json:"sdp"`
				ServiceID string `json:"serviceId"`
			}
			if err := json.NewDecoder(req.Body).Decode(&offerReq); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}
			regService := r.Services().Get(offerReq.ServiceID)
			if regService == nil {
				http.Error(w, "Service not found", http.StatusNotFound)
				return
			}
			if regService.NodeID != nodeID {
				http.Error(w, "Service does not belong to this node", http.StatusForbidden)
				return
			}
			_, answerSDP, err := sessionManager.NewSession(offerReq.SDP, regService.Service, r)
			if err != nil {
				http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"type": "answer",
				"sdp":  answerSDP,
			})

		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	go http.Serve(listener, mux)
	return listener, nil
}

// TestWebRTCIntegrationMJPEG tests WebRTC streaming from MJPEG camera through the relay
func TestWebRTCIntegrationMJPEG(t *testing.T) {
	// Load real services from allowlist.json
	allowList, err := node.LoadAllowList("../allowlist.json")
	if err != nil {
		t.Fatalf("Failed to load allowlist.json: %v", err)
	}

	// Find MJPEG service
	var mjpegService *node.Service
	for i, svc := range allowList.Services {
		if svc.Type == "mjpeg" {
			mjpegService = &allowList.Services[i]
			break
		}
	}

	if mjpegService == nil {
		t.Skip("No MJPEG service found in allowlist.json")
	}

	log.Printf("[Test] Using MJPEG service: %s at %s:%d%s", mjpegService.ID, mjpegService.Addr, mjpegService.Port, mjpegService.Path)

	// Start relay
	r := relay.NewRelay()
	relayAddr := "127.0.0.1:0"
	if err := r.Listen(relayAddr); err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer r.Shutdown()

	go r.Serve()

	actualRelayAddr := r.Addr().String()
	log.Printf("[Test] Relay started at %s", actualRelayAddr)

	// Start HTTP API on random port
	httpAPIAddr := "127.0.0.1:0"
	httpListener, err := startHTTPAPIListener(r, httpAPIAddr)
	if err != nil {
		t.Fatalf("Failed to start HTTP API: %v", err)
	}
	defer httpListener.Close()

	httpAPIAddr = httpListener.Addr().String()
	log.Printf("[Test] HTTP API started at %s", httpAPIAddr)

	// Start node client and wait for connection_ready to get the nodeId
	testNodeID := "test-node-" + uuid.New().String()
	nodeClient := node.NewNodeClient(actualRelayAddr, allowList.Services, testNodeID, "")

	var nodeID string
	nodeClient.OnConnectionReady = func(id, url string) {
		nodeID = id
		log.Printf("[Test] Node connected with ID: %s", id)
	}

	// Create a minimal config for testing
	testConfig := &node.Config{
		NodeID:   testNodeID,
		Services: allowList.Services,
		Token:    "",
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := nodeClient.Run(testConfig); err != nil {
			log.Printf("[Test] Node client error: %v", err)
		}
	}()

	// Give node time to register and announce
	time.Sleep(500 * time.Millisecond)

	if nodeID == "" {
		t.Fatal("Node did not receive connection_ready")
	}

	// Verify services are registered
	services := r.Services().List()
	log.Printf("[Test] Relay has %d services registered", len(services))

	if len(services) == 0 {
		t.Fatal("No services registered in relay")
	}

	// Fetch available services from HTTP API using node-specific endpoint
	resp, err := http.Get(fmt.Sprintf("http://%s/node/%s/services", httpAPIAddr, nodeID))
	if err != nil {
		t.Fatalf("Failed to fetch services: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[Test] Services response: %s", string(body))

	// Create WebRTC API (client side)
	api, err := webrtc.NewAPI()
	if err != nil {
		t.Fatalf("Failed to create WebRTC API: %v", err)
	}

	// Create peer connection (client side - browser)
	pc, err := api.NewPeerConnection(pionwebrtc.Configuration{
		ICEServers: []pionwebrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create peer connection: %v", err)
	}
	defer pc.Close()

	// Track incoming RTP packets
	var packetCount int
	var totalBytes int

	pc.OnTrack(func(track *pionwebrtc.TrackRemote, receiver *pionwebrtc.RTPReceiver) {
		codec := track.Codec()
		log.Printf("[Test] Got track: %s, codec: %s", track.Kind(), codec.MimeType)
		if track.Kind() == pionwebrtc.RTPCodecTypeVideo {
			go func() {
				buf := make([]byte, 1500)
				for {
					// Read from track, not receiver (receiver gives RTCP, track gives RTP video)
					n, _, err := track.Read(buf)
					if err != nil {
						if err != io.EOF {
							log.Printf("[Test] Read error: %v", err)
						}
						return
					}
					packetCount++
					totalBytes += n

					if packetCount <= 5 {
						log.Printf("[Test] Packet #%d: %d bytes", packetCount, n)
					}
				}
			}()
		}
	})

	pc.OnICEConnectionStateChange(func(state pionwebrtc.ICEConnectionState) {
		log.Printf("[Test] ICE state: %s", state)
	})

	// Add transceiver for video (recvonly - browser receives from server)
	pc.AddTransceiverFromKind(pionwebrtc.RTPCodecTypeVideo, pionwebrtc.RTPTransceiverInit{
		Direction: pionwebrtc.RTPTransceiverDirectionRecvonly,
	})

	// Create offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("Failed to create offer: %v", err)
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatalf("Failed to set local description: %v", err)
	}

	log.Printf("[Test] Created offer, sending to HTTP API...")

	// Send offer to HTTP API using node-specific endpoint
	offerReq := map[string]string{
		"sdp":       offer.SDP,
		"serviceId": mjpegService.ID,
	}

	offerBody, _ := json.Marshal(offerReq)
	resp, err = http.Post(
		fmt.Sprintf("http://%s/node/%s/offer", httpAPIAddr, nodeID),
		"application/json",
		strings.NewReader(string(offerBody)),
	)
	if err != nil {
		t.Fatalf("Failed to send offer: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse answer
	var answerResp struct {
		SDP string `json:"sdp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answerResp); err != nil {
		t.Fatalf("Failed to decode answer: %v", err)
	}

	log.Printf("[Test] Received answer")

	// Set remote description
	answer := pionwebrtc.SessionDescription{
		Type: pionwebrtc.SDPTypeAnswer,
		SDP:  answerResp.SDP,
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		t.Fatalf("Failed to set remote description: %v", err)
	}

	log.Printf("[Test] Waiting for RTP packets...")

	// Wait for packets
	time.Sleep(3 * time.Second)

	log.Printf("[Test] Received %d packets, total %d bytes", packetCount, totalBytes)

	if packetCount == 0 {
		t.Error("No packets received via WebRTC")
	}

	// Cleanup
	nodeClient.Close()
	wg.Wait()

	log.Printf("[Test] WebRTC MJPEG integration test completed")
}
