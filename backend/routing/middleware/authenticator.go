package middleware

import (
	"aura/config"
	"aura/logging"
	routes_auth "aura/routing/auth"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/jwtauth/v5"
)

// Authenticator is a middleware that checks for a valid JWT token, supplied either in the
// Authorization header or in the session cookie.
// If the token is valid, it calls the next handler; otherwise, it responds with a 401 Unauthorized error.
//
// It skips authentication for the following routes:
//   - /api/login
//   - /api/mediaserver/image/*
//   - /api/mediux/image/*
//
// If authentication is globally disabled in the configuration, it allows all requests to pass through.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if auth globally disabled
		if !config.Current.Auth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Public login route
		if strings.HasPrefix(r.URL.Path, "/api/login") ||
			strings.HasPrefix(r.URL.Path, "/api/images") ||
			strings.HasPrefix(r.URL.Path, "/api/sonarr/webhook") ||
			strings.HasPrefix(r.URL.Path, "/api/radarr/webhook") {
			next.ServeHTTP(w, r)
			return
		}

		ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
		logAction := ld.AddAction("Authenticate Request", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, logAction)
		defer logAction.Complete()

		// Ensure TokenAuth is initialized
		if routes_auth.TokenAuth == nil {
			sendNotAuthenticatedResponse(w, "Auth not initialized")
			logAction.SetError("Auth not initialized", "The authentication system is not set up", nil)
			return
		}

		// jwtauth.Verifier MUST already have run to populate context
		_, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			sendNotAuthenticatedResponse(w, "Invalid or expired token")
			logAction.SetError("Invalid or expired token", err.Error(), nil)
			return
		}

		if sub, _ := claims["sub"].(string); sub == "" {
			sendNotAuthenticatedResponse(w, "Invalid token")
			logAction.SetError("Invalid token", "Token missing 'sub' claim", nil)
			return
		}

		if typ, _ := claims["typ"].(string); !routes_auth.IsSessionTokenType(typ) {
			sendNotAuthenticatedResponse(w, "Invalid token")
			logAction.SetError("Invalid token", "Token is not a session token", nil)
			return
		}

		// Token is valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
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
			ld.Route.Params = r.URL.Query()
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

	// Get the IP address of the client
	ip := Request.RemoteAddr
	if forwarded := Request.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded // If the X-Forwarded-For header is present, use it instead
	}

	// Remove the port number from the IP address
	ip = regexp.MustCompile(`:\d+$`).ReplaceAllString(ip, "")

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
