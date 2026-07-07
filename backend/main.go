// @title Aura API
// @version 1.0
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"aura/config"
	"aura/logging"
	"aura/notification"
	"aura/routing"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	APP_NAME    = "aura"
	APP_VERSION = "dev"
	AUTHOR      = "xmoosex"
	LICENSE     = "MIT"
	APP_PORT    = 8888
)

var activeHandler atomic.Value

// preflightRetryInterval is how long to wait between preflight retries when the
// config is valid but a dependency (media server / MediUX) was unreachable at
// startup. See retryPreFlightUntilReady.
const preflightRetryInterval = 30 * time.Second

func init() {
	if strings.HasSuffix(APP_VERSION, "dev") {
		logging.SetDevMode(true)
	}
}

func main() {
	// Serve immediately with onboarding/public routes first.
	config.AppFullyLoaded = false
	config.AppVersion = APP_VERSION
	config.AppLoadingStep = "Initializing Application"
	activeHandler.Store(routing.NewRouter())

	// Start API now (non-blocking for init pipeline).
	go startAPI()

	// Run startup pipeline in background.
	go func() {
		bootStrapSuccess := runBootstrap()

		// activateFullRoutes runs warmup, flips the app to "fully loaded", sends the
		// start notification, and swaps in the full router. It is guarded by a
		// sync.Once so warmup (which inits the DB, registers cron jobs, and starts
		// the Plex WebSocket) and the router swap happen exactly once, no matter
		// which path reaches it: the boot-time preflight retry loop or the
		// onboarding-finalization callback, which can race if the user completes
		// onboarding in the UI while a background retry is in flight.
		// activated is closed exactly once, from inside activateFullRoutes' sync.Once,
		// to tell the background preflight-retry loop to stop. The retry loop selects on
		// this channel rather than reading config.AppFullyLoaded directly, which would be
		// an unsynchronized cross-goroutine read of that package-level flag (a data race).
		activated := make(chan struct{})
		var activateOnce sync.Once
		activateFullRoutes := func() {
			activateOnce.Do(func() {
				warmupSuccess := runWarmup()
				if !warmupSuccess {
					logging.LOGGER.Fatal().Timestamp().Msg("Warmup failed. Exiting application.")
					os.Exit(1)
				}

				config.Valid = true
				config.AppFullyLoaded = true
				config.AppLoadingStep = "App Fully Loaded"
				// Send App Start Notification (only if not dev & notifications enabled)
				if !strings.Contains(APP_VERSION, "dev") &&
					config.Current.Notifications.Enabled {
					notification.SendAppStartNotification(APP_PORT, APP_NAME, APP_VERSION)
				} else {
					logging.LOGGER.Warn().Timestamp().Bool("notifications_enabled", config.Current.Notifications.Enabled).Bool("dev_version", strings.Contains(APP_VERSION, "dev")).Msg("App start notification not sent")
				}
				activeHandler.Store(routing.NewRouter()) // swap to full routes
				close(activated)                         // stop any in-flight retry loop
				logging.LOGGER.Info().Timestamp().Msg("Main routes active.")
			})
		}

		// Keep callback for onboarding finalization path.
		routing.OnboardingComplete = func() {
			if !runPreFlight() {
				logging.LOGGER.Error().Timestamp().Msg("Preflight failed during OnboardingComplete, not swapping routers")
				return
			}
			activateFullRoutes()
		}

		if !bootStrapSuccess {
			// Config not loaded/valid: onboarding mode remains active until the user
			// completes onboarding through the UI (which fires OnboardingComplete).
			activeHandler.Store(routing.NewRouter())
			return
		}

		// Config is valid. Attempt preflight; on success, activate the full routes.
		if runPreFlight() {
			activateFullRoutes()
			return
		}

		// Preflight failed even though the config is valid, which means a dependency
		// (media server or MediUX) was unreachable at boot. Stay on the onboarding
		// router but keep retrying in the background so the app self-heals once the
		// dependency returns, instead of staying stuck in onboarding-only mode until
		// a manual restart.
		config.Valid = false
		activeHandler.Store(routing.NewRouter()) // stays onboarding
		go retryPreFlightUntilReady(activateFullRoutes, activated)
	}()

	// Keep process alive while startAPI runs.
	select {}
}

// retryPreFlightUntilReady re-runs preflight on an interval until it succeeds,
// then activates the full routes. It exists so a media-server/MediUX outage at
// boot does not leave the app permanently stuck serving onboarding-only routes:
// with the config already valid, the only thing missing is a reachable
// dependency, so we keep polling until it comes back. It stops early if
// onboarding finalization has already brought the app fully online, signalled by
// the activated channel being closed (from inside activateFullRoutes' sync.Once).
func retryPreFlightUntilReady(activateFullRoutes func(), activated <-chan struct{}) {
	ticker := time.NewTicker(preflightRetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-activated:
			return
		case <-ticker.C:
			// Re-check before spending a preflight cycle, in case activation happened
			// via the onboarding callback between ticks.
			select {
			case <-activated:
				return
			default:
			}
			logging.LOGGER.Warn().Timestamp().Msg("Retrying preflight checks after earlier failure")
			if runPreFlight() {
				activateFullRoutes()
				return
			}
		}
	}
}
