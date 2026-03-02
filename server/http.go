package server

import (
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
