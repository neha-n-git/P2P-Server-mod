// Package auth provides authentication and session management for the P2P network.
// It uses bcrypt for secure password hashing and supports concurrent access
// through mutex-protected data structures.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user with their hashed credentials.
type User struct {
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    string `json:"created_at"`
}

// UserStore manages user credentials with thread-safe access.
// It persists user data to a JSON file and runs a background
// goroutine for periodic auto-saving.
type UserStore struct {
	users    map[string]*User
	mu       sync.RWMutex
	filePath string
	dirty    bool // tracks if there are unsaved changes
}

// NewUserStore creates a new UserStore, loading existing users from disk if available.
// It also launches a background goroutine that auto-saves every 30 seconds.
func NewUserStore(dataDir string) (*UserStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &UserStore{
		users:    make(map[string]*User),
		filePath: filepath.Join(dataDir, "users.json"),
	}

	// Load existing users from file if it exists
	if err := store.loadFromFile(); err != nil {
		log.Printf("[Auth] Warning: could not load users file: %v (starting fresh)", err)
	}

	// Launch background auto-save goroutine
	go store.autoSaveLoop()

	return store, nil
}

// HashPassword generates a bcrypt hash from a plaintext password.
// Uses the default cost factor (10) for a balance of security and performance.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
// Returns true if they match, false otherwise.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// RegisterUser creates a new user with the given credentials.
// The password is hashed using bcrypt before storage.
// Returns an error if the username already exists or if the password is too short.
func (s *UserStore) RegisterUser(username, displayName, password string) error {
	if len(username) < 2 {
		return errors.New("username must be at least 2 characters")
	}
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	if displayName == "" {
		displayName = username
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return fmt.Errorf("username '%s' is already taken", username)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	s.users[username] = &User{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	s.dirty = true

	log.Printf("[Auth] User registered: %s (%s)", username, displayName)
	return nil
}

// AuthenticateUser validates a username/password combination.
// Returns the User if credentials are valid, nil otherwise.
func (s *UserStore) AuthenticateUser(username, password string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return nil
	}

	if !CheckPassword(user.PasswordHash, password) {
		return nil
	}

	return user
}

// GetUser retrieves a user by username (thread-safe).
func (s *UserStore) GetUser(username string) *User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, exists := s.users[username]
	if !exists {
		return nil
	}
	// Return a copy to prevent external mutation
	copy := *user
	return &copy
}

// UserCount returns the number of registered users.
func (s *UserStore) UserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// SaveToFile persists all user data to the JSON file on disk.
func (s *UserStore) SaveToFile() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.users, "", "  ")
	s.dirty = false
	s.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}

	return nil
}

// loadFromFile reads user data from the JSON file on disk.
func (s *UserStore) loadFromFile() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file yet, that's fine
		}
		return err
	}

	var users map[string]*User
	if err := json.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("failed to parse users file: %w", err)
	}

	s.users = users
	log.Printf("[Auth] Loaded %d users from %s", len(users), s.filePath)
	return nil
}

// autoSaveLoop runs in a goroutine and periodically saves dirty data to disk.
// Uses time.Ticker for efficient periodic execution.
func (s *UserStore) autoSaveLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		needsSave := s.dirty
		s.mu.RUnlock()

		if needsSave {
			if err := s.SaveToFile(); err != nil {
				log.Printf("[Auth] Auto-save error: %v", err)
			} else {
				log.Printf("[Auth] Auto-saved user data to disk")
			}
		}
	}
}
