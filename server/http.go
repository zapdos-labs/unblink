package server

import (
	"fmt"
	"log"
	"net/http"
)

// HandleNodeConnect handles WebSocket connections from nodes
func (s *Server) HandleNodeConnect(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	wsConn, err := s.UpgradeToWebSocket(w, r)
	if err != nil {
		log.Printf("[HTTP] WebSocket upgrade failed: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}

	log.Printf("[HTTP] Node connected from %s", r.RemoteAddr)

	// Create node connection handler
	nodeConn := NewNodeConn(wsConn, s)

	// Start handling the connection (non-blocking)
	go func() {
		if err := nodeConn.Run(); err != nil {
			log.Printf("[HTTP] Node connection error: %v", err)
		}
	}()
}

// GetHTTPHandler returns the HTTP handler with all endpoints
func (s *Server) GetHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// Node WebSocket endpoint
	mux.HandleFunc("/node/connect", s.HandleNodeConnect)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Root endpoint with basic info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><h1>Unb Server</h1><p>Node endpoint: <code>/node/connect</code></p></body></html>`)
	})

	return mux
}
