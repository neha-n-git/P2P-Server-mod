package auth

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// NetworkAuth handles network-level authentication for the P2P swarm.
// Each P2P network has a shared password that peers must provide when registering.
// The password is stored as a bcrypt hash for security.
type NetworkAuth struct {
	passwordHash string
	enabled      bool
}

// NewNetworkAuth creates a new NetworkAuth instance.
// If the password is empty, network authentication is disabled.
func NewNetworkAuth(password string) *NetworkAuth {
	if password == "" {
		log.Println("[NetworkAuth] No network password set — peer registration is open")
		return &NetworkAuth{enabled: false}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[NetworkAuth] Error hashing network password: %v", err)
		return &NetworkAuth{enabled: false}
	}

	log.Println("[NetworkAuth] Network password configured — peers must authenticate to join")
	return &NetworkAuth{
		passwordHash: string(hash),
		enabled:      true,
	}
}

// ValidateNetworkPassword checks if the provided password matches the network password.
// Returns true if authentication is disabled (open network) or if the password matches.
func (na *NetworkAuth) ValidateNetworkPassword(password string) bool {
	if !na.enabled {
		return true // open network
	}

	err := bcrypt.CompareHashAndPassword([]byte(na.passwordHash), []byte(password))
	return err == nil
}

// IsEnabled returns whether network authentication is active.
func (na *NetworkAuth) IsEnabled() bool {
	return na.enabled
}
