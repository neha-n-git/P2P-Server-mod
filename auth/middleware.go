package auth

import (
	"net/http"
)

// RequireAuth is middleware that protects routes by checking for a valid session.
// If no valid session is found, the user is redirected to the login page.
// For API routes (paths starting with /api/), it returns 401 JSON instead of redirecting.
func RequireAuth(next http.HandlerFunc, sm *SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			redirectOrReject(w, r)
			return
		}

		session, valid := sm.ValidateSession(cookie.Value)
		if !valid || session == nil {
			// Clear the invalid cookie
			http.SetCookie(w, &http.Cookie{
				Name:   "session_token",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			redirectOrReject(w, r)
			return
		}

		// Session is valid — proceed to the handler
		next(w, r)
	}
}

// redirectOrReject either redirects to login (for page requests) or returns 401 (for API requests).
func redirectOrReject(w http.ResponseWriter, r *http.Request) {
	// Check if this is an API request
	if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "authentication required"}`))
		return
	}

	// For page requests, redirect to login
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
