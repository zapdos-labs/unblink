package relay

import (
	"encoding/json"
	"log"
	"net/http"
)

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// MeResponse represents a /auth/me response
type MeResponse struct {
	Success bool   `json:"success"`
	User    *User  `json:"user,omitempty"`
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: false,
		Message: message,
	})
}

// handleRegister handles user registration requests
func handleRegister(w http.ResponseWriter, r *http.Request, authStore *AuthStore) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeJSONError(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := authStore.Register(req.Email, req.Password, req.Name)
	if err != nil {
		log.Printf("[Auth] Registration failed for %s: %v", req.Email, err)
		writeJSONError(w, "Registration failed", http.StatusBadRequest)
		return
	}

	log.Printf("[Auth] User registered: %s (ID: %d)", req.Email, user.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Registration successful",
		User:    user,
	})
}

// handleLogin handles user login requests with session cookie
func handleLogin(w http.ResponseWriter, r *http.Request, authStore *AuthStore, sessionManager *SessionManager) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		writeJSONError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Authenticate
	user, err := authStore.Login(req.Email, req.Password)
	if err != nil {
		log.Printf("[Auth] Login failed for %s: %v", req.Email, err)
		writeJSONError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create session
	session, err := sessionManager.Create(user.ID)
	if err != nil {
		log.Printf("[Auth] Failed to create session for %s: %v", req.Email, err)
		writeJSONError(w, "Login failed", http.StatusInternalServerError)
		return
	}

	// Set httpOnly cookie (XSS protection)
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		HttpOnly: true,              // Prevent JavaScript access (XSS protection)
		Secure:   false,             // Set true in production with HTTPS
		SameSite: http.SameSiteStrictMode, // CSRF protection
	})

	log.Printf("[Auth] User logged in: %s (ID: %d, Session: %s)", req.Email, user.ID, session.ID[:8]+"...")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    user,
	})
}

// handleLogout handles user logout requests
func handleLogout(w http.ResponseWriter, r *http.Request, sessionManager *SessionManager) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie("session")
	if err == nil {
		// Delete session from database
		if err := sessionManager.Delete(cookie.Value); err != nil {
			log.Printf("[Auth] Failed to delete session: %v", err)
		}
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Delete immediately
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// handleMe handles requests to get the current authenticated user
func handleMe(w http.ResponseWriter, r *http.Request, sessionManager *SessionManager, authStore *AuthStore) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie("session")
	if err != nil {
		writeJSONError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Validate session
	session, err := sessionManager.Validate(cookie.Value)
	if err != nil {
		writeJSONError(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	// Get user details
	user, err := authStore.GetUserByID(session.UserID)
	if err != nil {
		writeJSONError(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MeResponse{
		Success: true,
		User:    user,
	})
}
