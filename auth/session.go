package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"
)

// Session represents an authenticated user session.
type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionManager handles creation, validation, and cleanup of user sessions.
// It uses an in-memory map protected by a RWMutex for concurrent access.
// A background goroutine periodically reaps expired sessions.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	duration time.Duration // how long sessions last
}

// NewSessionManager creates a new SessionManager with the given session duration.
// It launches a background goroutine that reaps expired sessions every 5 minutes.
func NewSessionManager(sessionDuration time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		duration: sessionDuration,
	}

	// Launch background session reaper goroutine
	go sm.reapExpiredSessions()

	return sm
}

// generateToken creates a cryptographically secure random token.
// Uses crypto/rand for security-grade randomness.
func generateToken() (string, error) {
	bytes := make([]byte, 32) // 256-bit token
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateSession creates a new session for the given username.
// Returns the session token string to be stored in a cookie.
func (sm *SessionManager) CreateSession(username string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}

	now := time.Now()
	session := &Session{
		Token:     token,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.duration),
	}

	sm.mu.Lock()
	sm.sessions[token] = session
	sm.mu.Unlock()

	log.Printf("[Session] Created session for user '%s' (expires: %s)", username, session.ExpiresAt.Format(time.RFC3339))
	return token, nil
}

// ValidateSession checks if a session token is valid and not expired.
// Returns the Session if valid, nil and false otherwise.
func (sm *SessionManager) ValidateSession(token string) (*Session, bool) {
	sm.mu.RLock()
	session, exists := sm.sessions[token]
	sm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		sm.DestroySession(token)
		return nil, false
	}

	return session, true
}

// DestroySession removes a session by its token.
func (sm *SessionManager) DestroySession(token string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[token]; exists {
		log.Printf("[Session] Destroyed session for user '%s'", session.Username)
		delete(sm.sessions, token)
	}
}

// ActiveSessionCount returns the number of active sessions.
func (sm *SessionManager) ActiveSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// reapExpiredSessions runs in a goroutine and periodically removes expired sessions.
// Uses time.Ticker for efficient periodic cleanup.
func (sm *SessionManager) reapExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		reaped := 0
		for token, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, token)
				reaped++
			}
		}
		sm.mu.Unlock()

		if reaped > 0 {
			log.Printf("[Session] Reaped %d expired session(s)", reaped)
		}
	}
}
