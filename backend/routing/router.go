package routing

import (
	"aura/database"
	"aura/logging"
	routes_auth "aura/routing/auth"
	routes_base "aura/routing/base"
	"aura/routing/middleware"
	"context"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
)

// OnboardingComplete can be set by main to perform:
// - validation preflight
// - DB init
// - cron start
// - router swap
var OnboardingComplete func()

// ensure finalize only runs once
var onboardingFinalizeOnce sync.Once

func NewRouter() *chi.Mux {
	// Create a new router
	r := chi.NewRouter()

	// Configure the router with middlewares
	middleware.Configure(r)

	// Initialize Token Auth
	// The secret is stored in the database, which isn't open yet when the first router is created.
	// The routes are created again once it is.
	if database.Client == nil {
		logging.LOGGER.Debug().Timestamp().Msg("Database not open yet, JWT Token Auth is initialized once it is")
	} else if secret, Err := routes_auth.GetTokenAuthSecret(); Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Msg("Failed to get JWT Token Auth Secret: " + Err.Message)
	} else {
		routes_auth.SetTokenAuth(jwtauth.New("HS256", []byte(secret), nil))
		logging.LOGGER.Info().Timestamp().Msg("JWT Token Auth initialized successfully")
	}

	// Initialize OIDC (no-op if disabled). Bounded timeout so a slow/unreachable IdP can't hang
	// router (re)creation - if discovery fails, OIDC login is simply left unavailable (InitOIDC logs why).
	oidcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = routes_auth.InitOIDC(oidcCtx)
	cancel()

	// Add the routes to the router
	AddRoutes(r)

	// If the route is not found, return a JSON response
	r.NotFound(routes_base.NotFound)
	r.MethodNotAllowed(routes_base.MethodNotAllowed)

	return r
}
