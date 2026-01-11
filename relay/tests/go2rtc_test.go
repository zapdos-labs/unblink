package relay_test

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/rtsp"
	"github.com/google/uuid"
	"github.com/zapdos-labs/unblink/node"
	"github.com/zapdos-labs/unblink/relay"
)

// TestGo2RTCIntegration tests RTSP streaming through the relay bridge using go2rtc library
func TestGo2RTCIntegration(t *testing.T) {
	// Load real services from allowlist.json
	allowList, err := node.LoadAllowList("../allowlist.json")
	if err != nil {
		t.Fatalf("Failed to load allowlist.json: %v", err)
	}

	// Find RTSP service
	var rtspService *node.Service
	for i, svc := range allowList.Services {
		if svc.Type == "rtsp" {
			rtspService = &allowList.Services[i]
			break
		}
	}

	if rtspService == nil {
		t.Skip("No RTSP service found in allowlist.json")
	}

	log.Printf("[Test] Using RTSP service: %s at %s:%d%s", rtspService.ID, rtspService.Addr, rtspService.Port, rtspService.Path)

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

	// Start node client and get nodeID from connection_ready
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

	// Find the node connection
	nc := r.GetNode(nodeID)
	if nc == nil {
		t.Fatal("Node not registered")
	}

	// Open bridge to RTSP service
	log.Printf("[Test] Opening bridge to RTSP service: %s", rtspService.ID)
	bridgeID, err := nc.OpenBridge(*rtspService)
	if err != nil {
		t.Fatalf("Failed to open bridge: %v", err)
	}
	defer nc.CloseBridge(bridgeID)

	log.Printf("[Test] Bridge opened: %s", bridgeID)

	// Create a local TCP proxy that exposes the bridge as a local port
	proxy, err := relay.NewBridgeTCPProxy(nc, bridgeID, *rtspService)
	if err != nil {
		t.Fatalf("Failed to create bridge TCP proxy: %v", err)
	}
	defer proxy.Close()

	proxyAddr := proxy.Addr()
	log.Printf("[Test] Bridge proxy listening on %s", proxyAddr)

	// Build RTSP URL pointing to our local proxy
	// go2rtc will connect to localhost, and traffic goes through the bridge to the camera
	var rtspURL string
	if rtspService.Auth != nil {
		rtspURL = "rtsp://" + rtspService.Auth.Username + ":" + rtspService.Auth.Password + "@" + proxyAddr + rtspService.Path
	} else {
		rtspURL = "rtsp://" + proxyAddr + rtspService.Path
	}

	log.Printf("[Test] go2rtc connecting to: %s", rtspURL)

	// Create go2rtc RTSP client with TCP transport (interleaved mode)
	client := rtsp.NewClient(rtspURL)
	client.Transport = "tcp" // Force TCP interleaved mode

	// Connect using go2rtc's Dial
	if err := client.Dial(); err != nil {
		t.Fatalf("go2rtc Dial failed: %v", err)
	}
	defer client.Close()

	log.Printf("[Test] go2rtc connected successfully")

	// Perform DESCRIBE to get SDP
	if err := client.Describe(); err != nil {
		t.Fatalf("go2rtc DESCRIBE failed: %v", err)
	}

	log.Printf("[Test] DESCRIBE succeeded")

	// Get media streams from SDP
	medias := client.GetMedias()
	log.Printf("[Test] Found %d media streams in SDP", len(medias))

	if len(medias) == 0 {
		t.Fatal("No media streams found in SDP")
	}

	// Print media information
	for i, media := range medias {
		log.Printf("[Test] Media %d: %s", i, media.String())
	}

	// Get receivers for each media track to track RTP packets
	// IMPORTANT: Call GetTrack BEFORE Play(), otherwise it will fail with "wrong transport"
	// go2rtc delivers RTP packets to Receivers, not via the Listen() callback
	var totalPackets int
	var totalBytes int

	receivers := make(map[string]*core.Receiver)
	for _, media := range medias {
		for _, codec := range media.Codecs {
			receiver, err := client.GetTrack(media, codec)
			if err != nil {
				log.Printf("[Test] GetTrack error for %s/%s: %v", media.Kind, codec.Name, err)
				continue
			}
			if receiver != nil {
				key := media.Kind + "/" + codec.Name
				receivers[key] = receiver
				log.Printf("[Test] Track %s: receiver created (ID=%d, codec=%s)", key, receiver.ID, receiver.Codec.Name)
			}
		}
	}

	// Debug: check how many receivers total
	log.Printf("[Test] Total receivers created: %d", len(receivers))

	// Start PLAY
	if err := client.Play(); err != nil {
		t.Fatalf("go2rtc PLAY failed: %v", err)
	}

	log.Printf("[Test] PLAY succeeded, streaming started")

	// Start handling incoming data
	go func() {
		if err := client.Handle(); err != nil {
			log.Printf("[Test] Handle error: %v", err)
		}
	}()

	// Wait for packets
	log.Printf("[Test] Waiting for RTP packets from go2rtc...")
	time.Sleep(3 * time.Second)

	// Check packet counts from receivers
	for key, receiver := range receivers {
		totalPackets += receiver.Packets
		totalBytes += receiver.Bytes
		log.Printf("[Test] Track %s: %d packets, %d bytes",
			key, receiver.Packets, receiver.Bytes)
	}

	log.Printf("[Test] Total: %d packets, %d bytes from go2rtc", totalPackets, totalBytes)

	if totalPackets == 0 {
		t.Fatal("No packets received via go2rtc receivers")
	}

	// Teardown
	log.Printf("[Test] Sending TEARDOWN")
	if err := client.Teardown(); err != nil {
		log.Printf("[Test] TEARDOWN warning: %v", err)
	}

	// Close bridge
	nc.CloseBridge(bridgeID)
	log.Printf("[Test] Bridge closed")

	// Cleanup
	nodeClient.Close()
	wg.Wait()

	log.Printf("[Test] go2rtc integration test completed successfully")
}
