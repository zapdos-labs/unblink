package relay

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/unblink/unblink/node"
)

// StartWebSocketServerAsync starts the WebSocket server for node and worker connections and returns the server
func StartWebSocketServerAsync(r *Relay, addr string) (*http.Server, error) {
	mux := http.NewServeMux()

	// Node connections
	mux.HandleFunc("/node/connect", r.handleNodeWebSocket)

	// Worker connections
	mux.HandleFunc("/worker/connect", r.cvWorkerRegistry.HandleWebSocket)
	mux.HandleFunc("/worker/frames/", r.storageManager.HandleFrameDownload)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[Relay] WebSocket server listening on %s (nodes + workers)", addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Relay] WebSocket server error: %v", err)
		}
	}()

	return server, nil
}

// handleNodeWebSocket handles WebSocket upgrade requests from nodes
func (r *Relay) handleNodeWebSocket(w http.ResponseWriter, req *http.Request) {
	// Configure WebSocket upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: Add origin validation for production
			// For now, allow all origins since nodes connect directly
			return true
		},
	}

	// Upgrade HTTP connection to WebSocket
	wsConn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("[Relay] WebSocket upgrade failed from %s: %v", req.RemoteAddr, err)
		return
	}

	// Wrap WebSocket connection in our adapter
	transport := node.NewWebSocketConn(wsConn)

	// Create NodeConn (same structure as TCP)
	nodeConn := &NodeConn{
		conn:        transport,
		relay:       r,
		bridges:     make(map[string]*Bridge),
		bridgeChans: make(map[string]chan []byte),
		shutdown:    make(chan struct{}),
	}

	log.Printf("[Relay] WebSocket connection from %s", req.RemoteAddr)

	// Run protocol handler (unchanged from TCP version)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := nodeConn.Run(); err != nil {
			log.Printf("[Relay] Connection error: %v", err)
		}
		r.removeNode(nodeConn)
		log.Printf("[Relay] WebSocket connection closed from %s", req.RemoteAddr)
	}()
}
