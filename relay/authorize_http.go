package relay

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleAuthorizeAPI processes the authorization request
func handleAuthorizeAPI(w http.ResponseWriter, r *http.Request, sessionManager *SessionManager, relay *Relay) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authentication via session cookie
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	session, err := sessionManager.Validate(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	// Parse JSON request
	var req struct {
		NodeID string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.NodeID == "" {
		http.Error(w, "Missing node_id", http.StatusBadRequest)
		return
	}

	// Check if node is already authorized
	var ownerID interface{}
	err = relay.db.DB.QueryRow("SELECT owner_id FROM nodes WHERE id = ?", req.NodeID).Scan(&ownerID)
	if err == nil && ownerID != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Node already authorized",
			"node_id": req.NodeID,
		})
		return
	}

	// Generate token
	token, err := generateSecureToken()
	if err != nil {
		log.Printf("[HTTP] Failed to generate token: %v", err)
		http.Error(w, "Authorization failed", http.StatusInternalServerError)
		return
	}

	// Insert node with token and owner
	_, err = relay.db.DB.Exec(
		"INSERT INTO nodes (id, token, owner_id, name, authorized_at) VALUES (?, ?, ?, ?, datetime('now'))",
		req.NodeID, token, session.UserID, nil,
	)
	if err != nil {
		log.Printf("[HTTP] Failed to insert node: %v", err)
		http.Error(w, "Authorization failed", http.StatusInternalServerError)
		return
	}

	// Send token to node via TCP connection
	relay.SendTokenToNode(req.NodeID, token)

	log.Printf("[HTTP] Node %s authorized by user %d", req.NodeID, session.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Node authorized successfully",
		"node_id": req.NodeID,
	})
}

// requireAuth is a middleware that requires authentication
func requireAuth(w http.ResponseWriter, r *http.Request, sessionManager *SessionManager) (*Session, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return nil, err
	}

	session, err := sessionManager.Validate(cookie.Value)
	if err != nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return nil, err
	}
	return session, nil
}
