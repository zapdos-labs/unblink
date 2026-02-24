package webrtc

import (
	"log"
	"sync"
)

// SessionManager manages all active WebRTC sessions
type SessionManager struct {
	sessions map[string]*Session // sessionID -> Session
	sessionsMu sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Close closes the session manager and all its resources
func (m *SessionManager) Close() {
	m.CloseAll()
}

// AddSession adds a session to the manager
func (m *SessionManager) AddSession(session *Session) {
	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	m.sessions[session.SessionID] = session
	log.Printf("[SessionManager] Session %s added (total: %d)", session.SessionID, len(m.sessions))
}

// RemoveSession removes a session from the manager
func (m *SessionManager) RemoveSession(sessionID string) {
	m.sessionsMu.Lock()
	defer m.sessionsMu.Unlock()

	delete(m.sessions, sessionID)
	log.Printf("[SessionManager] Session %s removed (total: %d)", sessionID, len(m.sessions))
}

// GetSession retrieves a session by ID
func (m *SessionManager) GetSession(sessionID string) (*Session, bool) {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()

	session, exists := m.sessions[sessionID]
	return session, exists
}

// GetActiveSessions returns the count of active sessions
func (m *SessionManager) GetActiveSessions() int {
	m.sessionsMu.RLock()
	defer m.sessionsMu.RUnlock()

	return len(m.sessions)
}

// CloseAll closes all active sessions
func (m *SessionManager) CloseAll() {
	m.sessionsMu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessionsMu.Unlock()

	// Close all sessions outside the lock to avoid deadlock
	for _, session := range sessions {
		session.Close()
	}
}
