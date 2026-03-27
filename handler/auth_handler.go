package handler

import (
	"encoding/json"
	"net/http"
	"p2p/auth"
	"p2p/peer"
	"time"
)

// LoginPageHandler serves the login HTML page.
func LoginPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/login.html")
	}
}

// AuthLoginHandler handles POST requests for user authentication.
// Validates credentials against the UserStore using bcrypt,
// creates a session on success, and sets a session cookie.
func AuthLoginHandler(p *peer.Peer, store *auth.UserStore, sm *auth.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if payload.Username == "" || payload.Password == "" {
			jsonError(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Authenticate using bcrypt comparison
		user := store.AuthenticateUser(payload.Username, payload.Password)
		if user == nil {
			jsonError(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// Create a new session
		token, err := sm.CreateSession(user.Username)
		if err != nil {
			jsonError(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400, // 24 hours
			SameSite: http.SameSiteLaxMode,
		})

		// Set the active user on this peer for P2P visibility
		p.Mu.Lock()
		p.ActiveUser = user.DisplayName
		p.Mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "ok",
			"redirect":     "/",
			"username":     user.Username,
			"display_name": user.DisplayName,
		})
	}
}

// AuthRegisterHandler handles POST requests for new user registration.
// Validates input, creates user with bcrypt-hashed password, auto-logs in.
func AuthRegisterHandler(store *auth.UserStore, sm *auth.SessionManager, na *auth.NetworkAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Username        string `json:"username"`
			DisplayName     string `json:"display_name"`
			Password        string `json:"password"`
			ConfirmPassword string `json:"confirm_password"`
			NetworkPassword string `json:"network_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate network password if enabled
		if na.IsEnabled() && !na.ValidateNetworkPassword(payload.NetworkPassword) {
			jsonError(w, "Invalid network password — you cannot join this P2P network", http.StatusForbidden)
			return
		}

		// Validate password confirmation
		if payload.Password != payload.ConfirmPassword {
			jsonError(w, "Passwords do not match", http.StatusBadRequest)
			return
		}

		// Register user (bcrypt hashing happens inside)
		if err := store.RegisterUser(payload.Username, payload.DisplayName, payload.Password); err != nil {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}

		// Save to disk immediately after registration
		if err := store.SaveToFile(); err != nil {
			// Log but don't fail — auto-save will catch it
			_ = err
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "registered",
			"username":     payload.Username,
			"display_name": payload.DisplayName,
		})
	}
}

// AuthLogoutHandler handles POST requests to destroy the user session.
func AuthLogoutHandler(p *peer.Peer, sm *auth.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie("session_token")
		if err == nil {
			sm.DestroySession(cookie.Value)
			
			// Clear the active user on this peer
			p.Mu.Lock()
			p.ActiveUser = ""
			p.Mu.Unlock()
		}

		// Clear the cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"redirect": "/login",
		})
	}
}

// AuthStatusHandler returns the current user's info if authenticated.
func AuthStatusHandler(store *auth.UserStore, sm *auth.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := r.Cookie("session_token")
		if err != nil {
			jsonError(w, "Not authenticated", http.StatusUnauthorized)
			return
		}

		session, valid := sm.ValidateSession(cookie.Value)
		if !valid || session == nil {
			jsonError(w, "Session expired", http.StatusUnauthorized)
			return
		}

		user := store.GetUser(session.Username)
		if user == nil {
			jsonError(w, "User not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"username":     user.Username,
			"display_name": user.DisplayName,
			"created_at":   user.CreatedAt,
		})
	}
}

// jsonError sends a JSON error response.
func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
