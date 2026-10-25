package middleware

import (
	"aura/config"
	"aura/logging"
	routes_auth "aura/routing/auth"
	"aura/utils/httpx"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
)

// publicExactPaths lists routes that never require authentication, matched by exact path only
var publicExactPaths = []string{
	"/api",
	"/api/health",
	"/api/login",
	"/api/logout",
	"/api/config/auth-methods",
}

// publicPathPrefixes lists route *prefixes* that never require authentication
var publicPathPrefixes = []string{
	"/api/auth/oidc/",
	"/api/images/media/",
	"/api/images/mediux/",
	"/api/plugin/silo/download/", // Signed by aura, Silo can't send an API key
}

// Authenticator is a middleware that authenticates requests to protected routes using, in order:
//  1. Nothing, if Auth is globally disabled or the path is public (see isPublicPath).
//  2. HTTP Basic Auth (password checked as the API key) for the Sonarr/Radarr webhook route -
//     Sonarr/Radarr's built-in Webhook connection type only supports URL/Method/Username/Password,
//     not custom headers, so this is the only auth mechanism they can actually send.
//  3. An X-Api-Key header, verified against the configured API key hash. If present but invalid,
//     the request is rejected outright rather than silently falling back to a session cookie.
//  4. A valid aura_session browser cookie (signed JWT, set by password login or the OIDC callback).
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if auth globally disabled in the config, or with AURA_DISABLE_AUTH
		if !config.AuthEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
		logAction := ld.AddAction("Authenticate Request", logging.LevelInfo)
		logging.WithCurrentAction(ctx, logAction)
		defer logAction.Complete()

		if routes_auth.TokenAuth == nil {
			sendNotAuthenticatedResponse(w, "Auth not initialized")
			logAction.SetError("Auth not initialized", "The authentication system is not set up", nil)
			return
		}

		// Fork: the Radarr webhook authenticates the same way as Sonarr's.
		if strings.HasPrefix(r.URL.Path, "/api/sonarr/webhook") || strings.HasPrefix(r.URL.Path, "/api/radarr/webhook") {
			_, password, ok := r.BasicAuth()
			if !ok || !routes_auth.VerifyAPIKey(password) {
				sendNotAuthenticatedResponse(w, "Valid HTTP Basic Auth required (use the API key as the password)")
				logAction.SetError("Missing or invalid Basic Auth for webhook", "", nil)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if apiKey := r.Header.Get("X-Api-Key"); apiKey != "" {
			if !routes_auth.VerifyAPIKey(apiKey) {
				sendNotAuthenticatedResponse(w, "Invalid API key")
				logAction.SetError("Invalid API key", "", nil)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(routes_auth.SessionCookieName)
		if err != nil {
			sendNotAuthenticatedResponse(w, "Not authenticated")
			logAction.SetError("Missing session cookie", err.Error(), nil)
			return
		}

		token, err := jwtauth.VerifyToken(routes_auth.TokenAuth, cookie.Value)
		if err != nil {
			sendNotAuthenticatedResponse(w, "Invalid or expired session")
			logAction.SetError("Invalid or expired session", err.Error(), nil)
			return
		}

		if sub, ok := token.Subject(); !ok || sub == "" {
			sendNotAuthenticatedResponse(w, "Invalid session")
			logAction.SetError("Invalid session", "Token missing 'sub' claim", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPublicPath(path string) bool {
	if slices.Contains(publicExactPaths, path) {
		return true
	}
	for _, prefix := range publicPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

type responseWriterWithBytes struct {
	http.ResponseWriter
	bytesWritten int64
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		wrapped := &responseWriterWithBytes{ResponseWriter: w}

		ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)

		// Skip logging for certain paths/methods
		if logging.ShouldSkipLogging(r, ld) {
			next.ServeHTTP(w, r)
			return
		}

		ld.Route = &logging.LogRouteInfo{
			Method: r.Method,
			Path:   r.URL.Path,
			IP:     getLogIP(r),
		}

		if len(r.URL.Query()) > 0 {
			// Hide sensitive values like an OIDC ?code= in the logs
			ld.Route.Params = httpx.RedactQuery(r.URL.Query())
		}

		ctx = logging.WithLogData(r.Context(), ld)
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Set response bytes after handler
		ld.Route.ResponseBytes = wrapped.bytesWritten
		ld.Complete()
		ld.Log()
	})
}

func getLogIP(Request *http.Request) string {

	// Get the IP address of the client, as set by the Client IP middleware
	ip := middleware.GetClientIP(Request.Context())
	if ip == "" {
		// Remove the port number from the IP address
		ip = regexp.MustCompile(`:\d+$`).ReplaceAllString(Request.RemoteAddr, "")
	}

	// If the ip is ::1 (change it to localhost)
	if ip == "[::1]" || ip == "::1" {
		ip = "localhost"
	}

	// Return the IP address
	return ip
}

// sendNotAuthenticatedResponse sends a 401 Unauthorized response with a JSON message.
func sendNotAuthenticatedResponse(w http.ResponseWriter, message string) {
	resp := map[string]any{"message": message}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(resp)
}

func (w *responseWriterWithBytes) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

func (w *responseWriterWithBytes) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
