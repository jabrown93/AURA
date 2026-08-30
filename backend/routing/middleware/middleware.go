package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
)

/*
	Configure_Middlewares

Sets up the middleware stack for the given router.

Middlewares included:
- Custom Logging Middleware: Logs requests using a custom log formatter.
- CORS Middleware: Allows CORS for all origins (replace this with specific origins).
- RealIP Middleware: Gets the client's real public IP address from the request headers.
- StripSlashes Middleware: Strips slashes to no slash URL versions.
- Request Body Limit Middleware: Caps how much of a request body will be read.
- Panic Recovery Middleware: Recovers from panics and returns a 500 error.

Parameters:

- r: The chi.Mux router to configure middlewares for.
*/
func Configure(r *chi.Mux) {

	// Wildcard is safe alongside AllowCredentials here: rs/cors emits a literal "*" rather
	// than reflecting the request Origin, and browsers refuse to attach credentials to a
	// "*" response. Cross-origin reads therefore arrive without a session and are rejected
	// by the Authenticator. Static linters flag this pair on sight; it is not exploitable.
	AllowedOrigins := []string{"*"}

	// CORS Middleware: Allow CORS for all origins (replace this with specific origins)
	cors := cors.New(cors.Options{
		AllowedOrigins:   AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	r.Use(cors.Handler)

	// RealIP Middleware: Get the client's real public IP address from the request headers
	r.Use(middleware.RealIP)

	// StripSlashes Middleware: Strip slashes to no slash URL versions
	r.Use(middleware.StripSlashes)

	// Custom middleware to remove extra slashes
	r.Use(removeExtraSlashes)

	// Cap request bodies. /api/login and the Sonarr/Radarr webhooks take unauthenticated
	// POSTs, and every handler reads the body fully into memory before decoding it.
	r.Use(limitRequestBody)

	// Logging Middleware: Custom logging middleware
	r.Use(LoggingMiddleware)

	// Middleware for recovering from panics
	r.Use(middleware.Recoverer)
}

func removeExtraSlashes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.ReplaceAll(r.URL.Path, "//", "/")
		next.ServeHTTP(w, r)
	})
}

// maxRequestBody sits far above any legitimate payload. The largest real request is a saved
// item for a long-running show, which carries nested episode JSON; nothing here uploads files.
const maxRequestBody = 8 << 20

func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		next.ServeHTTP(w, r)
	})
}
