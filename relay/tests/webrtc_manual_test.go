package relay_test

import (
	"flag"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/unblink/unblink/node"
	"github.com/unblink/unblink/relay"
)

var (
	relayAddr   = flag.String("relay-addr", ":8080", "Relay listen address")
	httpAPIAddr = flag.String("http-addr", ":8081", "HTTP API listen address")
)

// TestWebRTCManual starts the relay, HTTP API, and a node for manual browser testing
//
// Usage: go test -v -run TestWebRTCManual -relay-addr=:8080 -http-addr=:8081
//
// Then open http://localhost:8081 (or the App.tsx dev server proxied to it)
func TestWebRTCManual(t *testing.T) {
	flag.Parse()

	// Load real services from allowlist.json
	allowList, err := node.LoadAllowList("../allowlist.json")
	if err != nil {
		t.Fatalf("Failed to load allowlist.json: %v", err)
	}

	log.Printf("[Manual] Starting relay on %s...", *relayAddr)

	// Start relay
	r := relay.NewRelay()
	if err := r.Listen(*relayAddr); err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer r.Shutdown()

	go r.Serve()

	log.Printf("[Manual] Relay started on %s", r.Addr().String())

	// Start HTTP API
	go func() {
		if err := relay.StartHTTPAPI(r, *httpAPIAddr); err != nil {
			log.Printf("[Manual] HTTP API error: %v", err)
		}
	}()

	log.Printf("[Manual] HTTP API started on http://%s", *httpAPIAddr)
	log.Printf("[Manual] Services endpoint: http://%s/services", *httpAPIAddr)
	log.Printf("[Manual] WebRTC offer endpoint: http://%s/offer", *httpAPIAddr)

	// Start node client
	testNodeID := "test-node-" + uuid.New().String()
	nodeClient := node.NewNodeClient(r.Addr().String(), allowList.Services, testNodeID, "")

	// Create a minimal config for testing
	testConfig := &node.Config{
		NodeID:   testNodeID,
		Services: allowList.Services,
		Token:    "",
	}

	go func() {
		if err := nodeClient.Run(testConfig); err != nil {
			log.Printf("[Manual] Node client error: %v", err)
		}
	}()

	// Wait for node to register
	time.Sleep(500 * time.Millisecond)

	services := r.Services().List()
	log.Printf("[Manual] Node registered with %d services:", len(services))
	for _, svc := range services {
		log.Printf("[Manual]   - %s (%s)", svc.Service.ID, svc.Service.Type)
	}

	log.Printf("[Manual]")
	log.Printf("[Manual] Ready for browser connections!")
	log.Printf("[Manual] Press Ctrl+C to stop...")
	log.Printf("[Manual]")

	// Keep test running indefinitely
	select {}
}
