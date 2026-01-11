package relay_test

import (
	"flag"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zapdos-labs/unblink/node"
	"github.com/zapdos-labs/unblink/relay"
)

var (
	mjpegRelayAddr   = flag.String("mjpeg-relay-addr", ":8080", "Relay listen address")
	mjpegHTTPAPIAddr = flag.String("mjpeg-http-addr", ":8081", "HTTP API listen address")
)

// TestWebRTCMJPEGManual starts the relay with MJPEG services for manual browser testing
//
// This test demonstrates MJPEG→H.264 transcoding via FFmpeg for WebRTC streaming.
//
// Usage: go test -v -run TestWebRTCMJPEGManual -mjpeg-relay-addr=:8080 -mjpeg-http-addr=:8081
//
// Then open a browser and use the WebRTC client to connect to an MJPEG service.
// You should see FFmpeg transcoding logs in the output.
func TestWebRTCMJPEGManual(t *testing.T) {
	flag.Parse()

	// Load services from allowlist.json
	allowList, err := node.LoadAllowList("../allowlist.json")
	if err != nil {
		t.Fatalf("Failed to load allowlist.json: %v", err)
	}

	// Filter to only MJPEG services
	var mjpegServices []node.Service
	for _, svc := range allowList.Services {
		if svc.Type == "mjpeg" {
			mjpegServices = append(mjpegServices, svc)
		}
	}

	if len(mjpegServices) == 0 {
		t.Fatal("No MJPEG services found in allowlist.json")
	}

	log.Printf("[MJPEG Manual] Found %d MJPEG services:", len(mjpegServices))
	for _, svc := range mjpegServices {
		log.Printf("[MJPEG Manual]   - %s: http://%s:%d%s", svc.ID, svc.Addr, svc.Port, svc.Path)
	}

	log.Printf("[MJPEG Manual]")
	log.Printf("[MJPEG Manual] Starting relay on %s...", *mjpegRelayAddr)

	// Start relay
	r := relay.NewRelay()
	if err := r.Listen(*mjpegRelayAddr); err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer r.Shutdown()

	go r.Serve()

	log.Printf("[MJPEG Manual] Relay started on %s", r.Addr().String())

	// Start HTTP API
	go func() {
		if err := relay.StartHTTPAPI(r, *mjpegHTTPAPIAddr); err != nil {
			log.Printf("[MJPEG Manual] HTTP API error: %v", err)
		}
	}()

	log.Printf("[MJPEG Manual] HTTP API started on http://%s", *mjpegHTTPAPIAddr)

	// Start node client with only MJPEG services
	testNodeID := "test-node-" + uuid.New().String()
	nodeClient := node.NewNodeClient(r.Addr().String(), mjpegServices, testNodeID, "")

	// Create a minimal config for testing
	testConfig := &node.Config{
		NodeID:   testNodeID,
		Services: mjpegServices,
		Token:    "",
	}

	go func() {
		if err := nodeClient.Run(testConfig); err != nil {
			log.Printf("[MJPEG Manual] Node client error: %v", err)
		}
	}()

	// Wait for node to register
	time.Sleep(500 * time.Millisecond)

	services := r.Services().List()
	log.Printf("[MJPEG Manual] Node registered with %d services", len(services))

	log.Printf("[MJPEG Manual]")
	log.Printf("[MJPEG Manual] ========================================")
	log.Printf("[MJPEG Manual] MJPEG→H.264 Transcoding Test Server")
	log.Printf("[MJPEG Manual] ========================================")
	log.Printf("[MJPEG Manual]")
	log.Printf("[MJPEG Manual] API Endpoints:")
	log.Printf("[MJPEG Manual]   GET  http://%s/relay/services  - List available services", *mjpegHTTPAPIAddr)
	log.Printf("[MJPEG Manual]   POST http://%s/relay/offer     - Send WebRTC offer", *mjpegHTTPAPIAddr)
	log.Printf("[MJPEG Manual]")
	log.Printf("[MJPEG Manual] When a browser connects to an MJPEG service, you will see:")
	log.Printf("[MJPEG Manual]   1. Bridge opened to camera")
	log.Printf("[MJPEG Manual]   2. FFmpeg starting MJPEG→H.264 transcoding")
	log.Printf("[MJPEG Manual]   3. WebRTC session established")
	log.Printf("[MJPEG Manual]   4. H.264 video streaming to browser")
	log.Printf("[MJPEG Manual]")
	log.Printf("[MJPEG Manual] Press Ctrl+C to stop...")
	log.Printf("[MJPEG Manual]")

	// Keep test running indefinitely
	select {}
}
