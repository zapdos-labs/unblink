package relay

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"time"
)

// SessionManager manages user sessions
type SessionManager struct {
	db *sql.DB
}

// Session represents a user session
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

// NewSessionManager creates a new session manager
func NewSessionManager(db *sql.DB) *SessionManager {
	return &SessionManager{db: db}
}

// generateSessionID generates a secure random session ID (32 bytes = 64 hex chars)
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Create creates a new session for a user (7 day expiry)
func (sm *SessionManager) Create(userID int64) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	_, err = sm.db.Exec(
		"INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)",
		id, userID, expiresAt,
	)
	if err != nil {
		return nil, err
	}

	log.Printf("[Session] Created session %s for user %d", id[:8]+"...", userID)

	return &Session{
		ID:        id,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

// Validate checks if a session is valid and not expired
func (sm *SessionManager) Validate(sessionID string) (*Session, error) {
	var session Session

	err := sm.db.QueryRow(
		"SELECT id, user_id, expires_at FROM sessions WHERE id = ? AND expires_at > ?",
		sessionID, time.Now(),
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid or expired session")
	}
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// Delete removes a session (for logout)
func (sm *SessionManager) Delete(sessionID string) error {
	result, err := sm.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[Session] Deleted session %s", sessionID[:8]+"...")
	}

	return nil
}

// CleanupExpired removes expired sessions (call periodically)
func (sm *SessionManager) CleanupExpired() error {
	result, err := sm.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", time.Now())
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[Session] Cleaned up %d expired sessions", rows)
	}

	return nil
}
