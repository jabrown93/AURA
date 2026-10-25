package middleware

import (
	"aura/config"
	"net/http"
	"slices"
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
- Client IP Middleware: Gets the client's IP address, using the X-Forwarded-For header when behind a reverse proxy.
- StripSlashes Middleware: Strips slashes to no slash URL versions.
- Panic Recovery Middleware: Recovers from panics and returns a 500 error.

Parameters:

- r: The chi.Mux router to configure middlewares for.
*/
func Configure(r *chi.Mux) {

	// CORS Middleware: the standard deployment (Next.js server proxying /api/* to this backend,
	// see frontend/next.config.ts rewrites) is same-origin from the browser's perspective and
	// never hits this at all - CORS only applies to genuine cross-origin requests. Wildcard +
	// AllowCredentials is also rejected by browsers per spec, so this must be an explicit list;
	// Auth.AllowedOrigins lets an admin opt a specific extra origin in if they're calling the API
	// directly from somewhere else. Read on every request, so changes apply without a restart.
	cors := cors.New(cors.Options{
		AllowOriginFunc: func(origin string) bool {
			return slices.Contains(config.Current.Auth.AllowedOrigins, origin)
		},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Api-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	r.Use(cors.Handler)

	// Client IP Middleware: Get the client's IP address from the connection,
	// or from the X-Forwarded-For header when the request comes from a reverse proxy
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(clientIPFromProxy)

	// StripSlashes Middleware: Strip slashes to no slash URL versions
	r.Use(middleware.StripSlashes)

	// Custom middleware to remove extra slashes
	r.Use(removeExtraSlashes)

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

// Private network ranges, where reverse proxies in front of aura are expected to be
var privateIPPrefixes = []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128", "fc00::/7"}

func clientIPFromProxy(next http.Handler) http.Handler {
	fromXFF := middleware.ClientIPFromXFF(privateIPPrefixes...)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := middleware.GetClientIPAddr(r.Context())
		if ip.IsPrivate() || ip.IsLoopback() {
			fromXFF.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
