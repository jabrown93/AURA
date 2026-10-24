// @title Aura API
// @version 1.0
// @BasePath /
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name aura_session
// @description Browser session cookie, set by POST /api/login or the OIDC callback. Not usable directly via Swagger "Authorize" - log in through the app UI in the same browser tab.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-Api-Key
// @description API key for programmatic/integration access. Generate one under Settings > Authentication, then send it as this header on every request. This is the intended auth method for scripts and integrations - the session cookie above is for browser use only.
// @securityDefinitions.basic BasicAuth
// @description Used only by /api/sonarr/webhook, since Sonarr/Radarr's built-in Webhook connection type has no custom-header support. Any username works; the password must be the API key.
package main

import (
	"aura/config"
	"aura/logging"
	"aura/notification"
	"aura/routing"
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	APP_NAME    = "aura"
	APP_VERSION = "dev"
	AUTHOR      = "xmoosex"
	LICENSE     = "MIT"
	APP_PORT    = 8888
)

// Set by the image build, empty for local builds
var (
	APP_COMMIT       = ""
	APP_BUILD_NUMBER = ""
	APP_BUILD_DATE   = ""
)

var activeHandler atomic.Value

func init() {
	if strings.HasSuffix(APP_VERSION, "dev") {
		logging.SetDevMode(true)
	}
}

func main() {
	// Shut down cleanly when aura is stopped
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Serve immediately with onboarding/public routes first.
	config.AppFullyLoaded = false
	config.AppVersion = APP_VERSION
	config.AppCommit, config.AppBuildNumber, config.AppBuildDate = APP_COMMIT, APP_BUILD_NUMBER, APP_BUILD_DATE
	config.SetAppLoadingStep("Initializing Application")
	activeHandler.Store(routing.NewRouter())

	// Start API now (non-blocking for init pipeline).
	go startAPI()

	// Run startup pipeline in background.
	go func() {
		bootStrapSuccess := runBootstrap()

		// Rebuild the routes now that the database is open and the config is loaded
		activeHandler.Store(routing.NewRouter())

		// Fork: warmup and the router swap run once, whichever path gets there first: boot, the
		// background preflight retry, or onboarding finishing in the UI while a retry is in flight.
		activated := make(chan struct{})
		var activateOnce sync.Once
		activate := func() {
			activateOnce.Do(func() {
				warmupSuccess := runWarmup()
				if !warmupSuccess {
					logging.LOGGER.Fatal().Timestamp().Msg("Warmup failed. Exiting application.")
					os.Exit(1)
				}
				config.Valid = true
				config.AppFullyLoaded = true
				activeHandler.Store(routing.NewRouter())
				config.SetAppLoadingStep("App Fully Loaded")
				config.NotifyStatusChanged()
				close(activated)
				logging.LOGGER.Info().Timestamp().Msg("Main routes active.")
			})
		}

		// Keep callback for onboarding finalization path.
		routing.OnboardingComplete = func() {
			preflightSuccess := runPreFlight()
			if !preflightSuccess {
				logging.LOGGER.Error().Timestamp().Msg("Preflight failed during OnboardingComplete, not swapping routers")
				return
			}
			activate()
		}

		if bootStrapSuccess {
			preflightSuccess := runPreFlight()
			if !preflightSuccess {
				config.Valid = false
				activeHandler.Store(routing.NewRouter()) // stays onboarding
				config.NotifyStatusChanged()
				// Fork: an outage at boot self-heals instead of waiting for a restart.
				if preflightFailedOnOutage() {
					go retryPreFlightUntilReady(activate, activated)
				}
				return
			}

			activate()
			// Send App Start Notification
			// Send notification (only if not dev & notifications enabled)
			if !strings.Contains(APP_VERSION, "dev") &&
				config.Current.Notifications.Enabled {
				notification.SendAppStartNotification(APP_PORT, APP_NAME, APP_VERSION)
			} else {
				logging.LOGGER.Warn().Timestamp().Bool("notifications_enabled", config.Current.Notifications.Enabled).Bool("dev_version", strings.Contains(APP_VERSION, "dev")).Msg("App start notification not sent")
			}
			return
		}

		// Config not loaded/valid: onboarding mode remains active.
		activeHandler.Store(routing.NewRouter())
		config.NotifyStatusChanged()
	}()

	// Keep running until aura is stopped
	<-stopCtx.Done()
	// A second Ctrl+C stops aura right away
	stop()
	shutdown()
}
