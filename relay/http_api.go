package relay

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func StartHTTPAPIServer(relay *Relay, addr string, cfg *Config) (*http.Server, error) {
	mux := http.NewServeMux()

	// Wrap with CORS middleware
	handler := corsMiddleware(mux)

	sessionManager := NewWebRTCSessionManager()
	authStore := NewAuthStore(relay.db.DB)

	mux.HandleFunc("/api/authorize", func(w http.ResponseWriter, r *http.Request) {
		handleAuthorizeAPI(w, r, cfg, relay)
	})

	// List all nodes (protected)
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		userID, err := requireAuth(w, r, cfg)
		if err != nil {
			return
		}
		handleListNodes(w, r, relay, userID)
	})

	// Node-specific endpoints: /node/{nodeId}/services and /node/{nodeId}/offer (protected)
	mux.HandleFunc("/node/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /node/{nodeId}/{endpoint}
		path := strings.TrimPrefix(r.URL.Path, "/node/")
		parts := strings.SplitN(path, "/", 2)

		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "Invalid path. Expected /node/{nodeId}/{services|offer}", http.StatusBadRequest)
			return
		}

		nodeID := parts[0]
		endpoint := parts[1]

		// Check authentication
		userID, err := requireAuth(w, r, cfg)
		if err != nil {
			return
		}

		// Verify node ownership
		if !relay.nodeTable.UserOwnsNode(userID, nodeID) {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}

		switch endpoint {
		case "services":
			handleNodeServices(w, r, relay, nodeID)
		case "offer":
			handleNodeOffer(w, r, relay, nodeID, sessionManager)
		case "name":
			handleUpdateNodeName(w, r, relay, nodeID)
		case "delete":
			handleDeleteNode(w, r, relay, nodeID)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})

	// Authentication endpoints
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		handleRegister(w, r, authStore)
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		handleLogin(w, r, authStore, cfg)
	})

	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		handleLogout(w, r)
	})

	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		handleMe(w, r, authStore, cfg)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	log.Printf("[HTTP] Starting HTTP API on %s", addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	return server, nil
}

// corsMiddleware adds CORS headers to all responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true") // Required for credentials mode

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// handleListNodes returns a list of nodes owned by the authenticated user
func handleListNodes(w http.ResponseWriter, r *http.Request, relay *Relay, userID int64) {

	// Get nodes owned by this user
	nodes, err := relay.nodeTable.GetNodesByUser(userID)
	if err != nil {
		log.Printf("[HTTP] handleListNodes: error getting nodes: %v", err)
		http.Error(w, "Failed to fetch nodes", http.StatusInternalServerError)
		return
	}

	// Return nodes with status information
	type NodeResponse struct {
		ID              string  `json:"id"`
		Status          string  `json:"status"`
		Name            *string `json:"name,omitempty"`
		LastConnectedAt *string `json:"last_connected_at,omitempty"`
	}

	result := make([]NodeResponse, len(nodes))
	for i, node := range nodes {
		// Check if node is currently connected
		status := "offline"
		if relay.GetNode(node.ID) != nil {
			status = "online"
		}

		var lastConnected *string
		if node.LastConnectedAt != nil {
			formatted := node.LastConnectedAt.Format("2006-01-02T15:04:05Z07:00")
			lastConnected = &formatted
		}

		result[i] = NodeResponse{
			ID:              node.ID,
			Status:          status,
			Name:            node.Name,
			LastConnectedAt: lastConnected,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleNodeServices returns services filtered by node ID
func handleNodeServices(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := relay.Services().ListByNode(nodeID)

	// Check if node exists (either connected or has registered services)
	node := relay.GetNode(nodeID)
	nodeExists := node != nil || len(services) > 0

	if !nodeExists {

		http.Error(w, "Node not found: "+nodeID, http.StatusNotFound)
		return
	}

	type ServiceInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
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
			Name:   s.Service.Name,
			Type:   s.Service.Type,
			NodeID: s.NodeID,
			Addr:   s.Service.Addr,
			Port:   s.Service.Port,
			Path:   s.Service.Path,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleNodeOffer handles WebRTC offers for a specific node's services
func handleNodeOffer(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string, sessionManager *WebRTCSessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SDP       string `json:"sdp"`
		ServiceID string `json:"serviceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[HTTP] WebRTC offer for service %s (node %s) from %s", req.ServiceID, nodeID, r.RemoteAddr)

	// Find service
	regService := relay.Services().Get(req.ServiceID)
	if regService == nil {
		http.Error(w, "Service not found: "+req.ServiceID, http.StatusNotFound)
		return
	}

	// Verify service belongs to this node
	if regService.NodeID != nodeID {
		http.Error(w, "Service does not belong to this node", http.StatusForbidden)
		return
	}

	service := regService.Service

	// Create WebRTC session
	sessionID, answerSDP, err := sessionManager.NewSession(req.SDP, service, relay)
	if err != nil {
		log.Printf("[HTTP] WebRTC session failed: %v", err)
		http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[HTTP] WebRTC session %s created for service %s", sessionID, req.ServiceID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"type": "answer",
		"sdp":  answerSDP,
	})
}

// handleUpdateNodeName handles updating a node's name
func handleUpdateNodeName(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate name
	if len(req.Name) > 255 {
		http.Error(w, "Name too long (max 255 characters)", http.StatusBadRequest)
		return
	}

	// Update node name
	if err := relay.nodeTable.UpdateNodeName(nodeID, req.Name); err != nil {
		log.Printf("[HTTP] Failed to update node name: %v", err)
		http.Error(w, "Failed to update node name", http.StatusInternalServerError)
		return
	}

	log.Printf("[HTTP] Updated name for node %s to '%s'", nodeID, req.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": "true",
		"name":    req.Name,
	})
}

// handleDeleteNode handles deleting/unauthorizing a node
func handleDeleteNode(w http.ResponseWriter, r *http.Request, relay *Relay, nodeID string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete node from database
	if err := relay.nodeTable.DeleteNode(nodeID); err != nil {
		log.Printf("[HTTP] Failed to delete node: %v", err)
		http.Error(w, "Failed to delete node", http.StatusInternalServerError)
		return
	}

	// If node is currently connected, disconnect it
	if nc := relay.GetNode(nodeID); nc != nil {
		nc.Close()
	}

	log.Printf("[HTTP] Deleted node %s", nodeID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": "true",
	})
}
