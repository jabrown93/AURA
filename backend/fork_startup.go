package main

import (
	"aura/anidb"
	"aura/config"
	"aura/jobs"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Fork: startup resilience for headless deployments, and an optional HTTPS listener.

// APP_TLS_PORT serves HTTPS when TLS_CERT_FILE and TLS_KEY_FILE are set.
const APP_TLS_PORT = 8443

// preflightRetryInterval is how long to wait between background preflight and MediUX rechecks.
const preflightRetryInterval = 30 * time.Second

var tlsServer *http.Server

// hardenServer bounds slow request headers and bodies. WriteTimeout stays unset: image responses
// are proxied from a media server that can be slow, and event streams stay open.
func hardenServer(s *http.Server) {
	s.ReadHeaderTimeout = 15 * time.Second
	s.ReadTimeout = 60 * time.Second
	s.IdleTimeout = 120 * time.Second
}

// startTLSIfConfigured adds an HTTPS listener beside the plain one, which the frontend's /api
// rewrite to http://localhost:8888 still needs. Certificates load once; rotating them needs a
// restart. Half-configured TLS is fatal, since silently serving HTTP only defeats asking for HTTPS.
func startTLSIfConfigured() {
	certFile, keyFile := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")
	if certFile == "" && keyFile == "" {
		return
	}
	if certFile == "" || keyFile == "" {
		logging.LOGGER.Fatal().Str("TLS_CERT_FILE", certFile).Str("TLS_KEY_FILE", keyFile).
			Msg("TLS_CERT_FILE and TLS_KEY_FILE must both be set to enable HTTPS")
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		logging.LOGGER.Fatal().Err(err).Msg("Failed to load TLS certificate/key pair")
	}
	tlsServer = &http.Server{Addr: fmt.Sprintf(":%d", APP_TLS_PORT), Handler: http.HandlerFunc(dispatch)}
	hardenServer(tlsServer)
	go func() {
		logging.LOGGER.Info().Timestamp().Int("port", APP_TLS_PORT).Msg("Starting HTTPS Server")
		if err := tlsServer.ListenAndServeTLS(certFile, keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.LOGGER.Fatal().Err(err).Msg("Failed to start HTTPS server")
		}
	}()
}

func shutdownTLS(ctx context.Context) {
	if tlsServer != nil {
		_ = tlsServer.Shutdown(ctx)
	}
}

// recordPreflightFailure tells "the service refused us" (a config problem the user must fix)
// apart from an outage, which is retried in the background instead of opening setup.
func recordPreflightFailure(Err logging.LogErrorInfo, valid, reachable *bool) {
	if isRejected(Err) {
		*valid = false
		*reachable = true
		return
	}
	// Not refused, so the config itself isn't wrong; keeping it valid keeps setup closed.
	*valid = true
	*reachable = false
}

// preflightFailedOnOutage reports whether a failed preflight only lacked a reachable service.
func preflightFailedOnOutage() bool {
	return config.MediaServerValid && config.MediuxValid
}

// retryPreFlightUntilReady re-runs preflight until it succeeds, then activates the full routes.
// Without it an outage at boot leaves the app on onboarding routes until a manual restart, as the
// onboarding callback never fires in a config-driven deployment.
func retryPreFlightUntilReady(activate func(), activated <-chan struct{}) {
	defer logging.RecoverPanic("Preflight Retry")
	ticker := time.NewTicker(preflightRetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-activated:
			return
		case <-ticker.C:
		}
		logging.LOGGER.Warn().Timestamp().Msg("Retrying preflight checks after earlier failure")
		if runPreFlight() {
			activate()
			return
		}
	}
}

// forkWarmup runs the fork's warmup steps. Called before the media-server library loads, since the
// Plex library path needs the AniDB mapping for HAMA items.
func forkWarmup(ctx context.Context) {
	if config.Current.MediaServer.Type == "Plex" {
		config.SetAppLoadingStep("Preloading AniDB Mappings into Cache")
		anidb.PreloadAnidbMappings(ctx)
	}
}

// startForkJobs schedules the fork-only jobs.
func startForkJobs() {
	for _, key := range []jobs.JobKey{jobs.JobKeyKometaImport, jobs.JobKeyAnidbRefresh} {
		if err := jobs.StartJob(key); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Str("job", string(key)).Msg("Failed to schedule job")
		}
	}
}

// recheckMediuxUntilReachable re-validates the MediUX token after a degraded start and rebuilds
// the MediUX-derived caches once it answers. It doesn't re-run warmup, which would duplicate jobs
// and listeners on an already-loaded app.
func recheckMediuxUntilReachable() {
	defer logging.RecoverPanic("MediUX Recheck")
	ticker := time.NewTicker(preflightRetryInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, ld := logging.CreateLoggingContext(context.Background(), "MediUX Recheck")
		action := ld.AddAction("Rechecking MediUX Availability", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, action)
		_, Err := mediux.ValidateToken(ctx, config.Current.Mediux.ApiToken)
		done := false
		switch {
		case Err.Message == "":
			config.MediuxReachable = true
			mediux.PreloadMediuxUsers(ctx)
			mediux.PreLoadMediuxItemsWithSets(ctx)
			_ = mediaserver.GetAllLibrarySectionsAndItems(ctx, true)
			config.NotifyStatusChanged()
			done = true
		case isRejected(Err):
			config.MediuxValid = false
			config.MediuxReachable = true
			config.NotifyStatusChanged()
			done = true
		}
		action.Complete()
		ld.Log()
		if done {
			return
		}
	}
}
