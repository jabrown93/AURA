package middleware

import (
	"aura/config"
	"aura/utils/httpx"
	"encoding/json"
	"net/http"
	"slices"
	"time"
)

// SetupCodeHeader is the header the setup page sends the setup code in
const SetupCodeHeader = "X-Setup-Code"

// Setup mode routes that never need the setup code
var setupPublicPaths = []string{
	"/api",
	"/api/health",
	"/api/config/auth-methods",
	"/api/setup/verify",
}

// SetupGuard protects the onboarding routes.
// If auth is enabled in the saved config, requests must be logged in.
// Otherwise, requests must include the setup code printed in the logs.
// GET /api/config only returns the app status without the code.
func SetupGuard(next http.Handler) http.Handler {
	authenticated := Authenticator(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.AuthEnabled() {
			authenticated.ServeHTTP(w, r)
			return
		}

		if slices.Contains(setupPublicPaths, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if config.CheckSetupCode(r.Header.Get(SetupCodeHeader)) {
			next.ServeHTTP(w, r.WithContext(httpx.WithSetupCodeVerified(r.Context())))
			return
		}

		// Both only return the app status without the setup code
		if r.Method == http.MethodGet && (r.URL.Path == "/api/config" || r.URL.Path == "/api/config/stream") {
			next.ServeHTTP(w, r)
			return
		}

		sendSetupCodeRequiredResponse(w)
	})
}

// sendSetupCodeRequiredResponse sends a 403 asking for the setup code.
// 403 instead of 401, so the frontend asks for the setup code instead of showing the login page.
func sendSetupCodeRequiredResponse(w http.ResponseWriter) {
	// Slow down guessing
	time.Sleep(500 * time.Millisecond)
	resp := map[string]any{
		"message":             "Setup code required",
		"help":                "Enter the setup code from the aura logs",
		"setup_code_required": true,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(resp)
}
