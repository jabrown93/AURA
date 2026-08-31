package main

import (
	"aura/anidb"
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/database/migration"
	autodownload "aura/download/auto"
	downloadqueue "aura/download/queue"
	"aura/jobs"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/utils"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"
)

func runBootstrap() (success bool) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Bootstrap")
	defer ld.Log()
	config.AppLoadingStep = "Bootstrapping Application"

	logAction := ld.AddAction("Application Startup", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	success = false

	// Print App Info
	utils.PrintAppStartUpDetails(APP_VERSION, AUTHOR, LICENSE, APP_PORT, APP_NAME)
	config.AppVersion = APP_VERSION

	// Set Umask for file permissions (if needed)
	config.AppLoadingStep = "Setting UMask for File Permissions"
	utils.SetUMask(ctx)

	// Load the config file
	config.AppLoadingStep = "Loading Configuration"
	config.LoadYAML(ctx)
	logAction.Complete()

	// Print the config details (sanitized)
	config.Current.PrintDetails()

	// If the config is loaded, validate it
	if config.Loaded {
		config.AppLoadingStep = "Validating Configuration"
		config.Current.Validate(ctx)
	}

	if config.Loaded && config.Valid {
		success = true
	}

	return success
}

func runPreFlight() (success bool) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Preflight")
	defer ld.Log()
	config.AppLoadingStep = "Performing Pre-Flight Checks"

	action := ld.AddAction("Checking Services", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	defer action.Complete()

	success = false

	// Note: config.AppFullyLoaded is intentionally NOT reset here. It is owned solely by
	// activateFullRoutes (main.go), only ever transitions false->true, and only once. It
	// is already false on every path that reaches preflight (boot, onboarding, retry).
	// Clobbering it back to false here would let a background preflight retry undo the
	// onboarding path's activation, stranding the UI on /app-loading even though full
	// routes are live.

	// Validate Media Server Connection
	config.AppLoadingStep = "Validating Media Server Connection"
	connectionOk, serverName, serverVersion, msErr := mediaserver.TestConnection(ctx, &config.Current.MediaServer)
	if msErr.Message != "" || !connectionOk || serverVersion == "" || serverName == "" {
		config.MediaServerValid = false
		config.MediaServerReachable = false
		return success
	}
	if config.Current.MediaServer.Type == "Jellyfin" || config.Current.MediaServer.Type == "Emby" {
		// Get Admin User for Emby/Jellyfin
		config.AppLoadingStep = "Retrieving Media Server Admin User"
		ejUserID, initErr := mediaserver.GetAdminUser(ctx, &config.Current.MediaServer)
		if initErr.Message != "" {
			config.MediaServerValid = false
			config.MediaServerReachable = false
			return success
		} else if ejUserID == "" {
			config.MediaServerValid = false
			config.MediaServerReachable = false
			logging.LOGGER.Error().Timestamp().Msg("Failed to retrieve admin user ID from Emby/Jellyfin server")
			return success
		}
		config.Current.MediaServer.UserID = ejUserID
	}
	config.MediaServerName = serverName
	logging.LOGGER.Trace().Timestamp().Str("media_server_name", serverName).
		Str("media_server_version", serverVersion).
		Msg("Media Server connection validated successfully")
	config.MediaServerValid = true
	config.MediaServerReachable = true

	// Validate MediUX Token
	config.AppLoadingStep = "Validating MediUX Token"
	if validateMediuxToken(ctx) == mediuxTokenRejected {
		// MediUX answered and refused the token, so the configured token is wrong.
		// Fail preflight to keep the app on onboarding until the user fixes it,
		// rather than starting degraded as we do for an outage.
		return success
	}

	if config.MediaServerValid || config.MediuxValid {
		success = true
	}

	return success
}

type mediuxTokenResult int

const (
	mediuxTokenAccepted mediuxTokenResult = iota
	mediuxTokenRejected
	mediuxUnreachable
)

// validateMediuxToken checks the configured MediUX token and records the outcome
// on the config package's flags. It separates "MediUX said no" from "MediUX did
// not answer": the first is a config problem the user must fix, the second is an
// outage the app can run through in a degraded state.
func validateMediuxToken(ctx context.Context) mediuxTokenResult {
	tokenValid, mediuxErr := mediux.ValidateToken(ctx, config.Current.Mediux.ApiToken)
	switch {
	case mediuxErr.Message == "" && tokenValid:
		config.MediuxValid = true
		config.MediuxReachable = true
		return mediuxTokenAccepted
	case mediuxAuthRejected(mediuxErr):
		config.MediuxValid = false
		config.MediuxReachable = true
		return mediuxTokenRejected
	default:
		config.MediuxValid = false
		config.MediuxReachable = false
		return mediuxUnreachable
	}
}

// mediuxAuthRejected reports whether MediUX explicitly refused the token rather
// than failing to answer. mediux.makeRequest records the response status under
// status_code in the error detail; a transport failure carries no status at all.
func mediuxAuthRejected(err logging.LogErrorInfo) bool {
	status, ok := err.Detail["status_code"].(int)
	return ok && (status == http.StatusUnauthorized || status == http.StatusForbidden)
}

func runWarmup() (success bool) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Warmup")

	action := ld.AddAction("Initializing Application", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	config.AppLoadingStep = "Warming Up Application"

	success = false

	// Cache: Add all MediUX users
	config.AppLoadingStep = "Preloading MediUX Users into Cache"
	mediux.PreloadMediuxUsers(ctx)

	// Cache: Get a list of all items in MediUX that has a set
	config.AppLoadingStep = "Preloading MediUX Items with Sets into Cache"
	mediux.PreLoadMediuxItemsWithSets(ctx)

	// Cache: Load AniDB -> TMDB mappings (Fribb anime-lists) so anime items that
	// Plex matched with the HAMA agent (AniDB IDs only) resolve to TMDB instead
	// of being dropped. Plex-only: the mapping is consumed solely by the Plex
	// library path (Emby/Jellyfin resolve via ProviderIds), so gating it here
	// keeps non-Plex startups from blocking on the Fribb fetch. Must run before
	// the media-server preload below.
	if config.Current.MediaServer.Type == "Plex" {
		config.AppLoadingStep = "Preloading AniDB Mappings into Cache"
		anidb.PreloadAnidbMappings(ctx)
	}

	// Database: Initialize
	config.AppLoadingStep = "Initializing Database"
	newDB, dbInitErr := database.Init(ctx)
	if dbInitErr.Message != "" {
		return false
	}
	logging.LOGGER.Info().Timestamp().Bool("new_database", newDB).Msg("Database initialized")

	// Database-Migration: If not a new DB, run migrations
	if !newDB {
		config.AppLoadingStep = "Running Database Migrations"
		migrationsCompleted, migrateErr := migration.RunMigrations()
		logging.LOGGER.Info().Timestamp().Msgf("%d database migrations performed", migrationsCompleted)
		if migrateErr.Message != "" {
			// A failed migration leaves the schema partially applied and the VERSION row
			// unbumped, so booting on would run every query against a schema that does not
			// match the code. Fail warmup instead of half-booting.
			config.AppLoadingStep = "Database Migration Failed"
			logging.LOGGER.Error().Timestamp().
				Str("help", migrateErr.Help).
				Interface("detail", migrateErr.Detail).
				Msgf("Database migration failed: %s", migrateErr.Message)
			return false
		}
	}

	// Cache: Add all media server sections and items
	config.AppLoadingStep = "Preloading Media Server Data into Cache"
	_ = mediaserver.GetAllLibrarySectionsAndItems(ctx, false)
	logging.LOGGER.Info().Timestamp().Int("sections", cache.LibraryStore.GetSectionsCount()).Int("items", cache.LibraryStore.GetItemsCount()).Msg("Loaded Media Server sections and items into cache")
	logging.LOGGER.Info().Timestamp().Int("collection_items", len(cache.CollectionsStore.GetAllCollections())).
		Msg("Loaded Media Server collections into cache")

	// Database: Vacuum
	config.AppLoadingStep = "Optimizing Database"
	vacuumErr := database.Vacuum(ctx)
	if vacuumErr.Message != "" {
		logging.LOGGER.Error().Timestamp().Msgf("Database VACUUM failed: %s", vacuumErr.Message)
		return false
	}

	action.Complete()
	ld.Log()

	// Cronjob: Auto Download Processing
	config.AppLoadingStep = "Starting Background Jobs"
	jobs.StartAutoDownloadJob()

	// Cronjob: Kometa Asset Import (only schedules if enabled + ImportCron set)
	if err := jobs.StartKometaImportJob(); err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Kometa Asset Import cron job")
	}

	// Cronjob: Download Queue Processing
	err := jobs.StartDownloadQueueJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Download Queue Processing cron job")
		downloadqueue.UpdateLatestInfo(func(info *downloadqueue.LatestInfo) {
			info.Time = time.Now()
			info.Status = downloadqueue.LAST_STATUS_ERROR
			info.Message = "Failed to schedule Download Queue Processing"
			info.Errors = []string{err.Error()}
			info.Warnings = []string{}
		})
	} else {
		downloadqueue.UpdateLatestInfo(func(info *downloadqueue.LatestInfo) {
			info.Time = time.Now()
			info.Status = downloadqueue.LAST_STATUS_IDLE
		})
	}

	// Cronjob: Refresh Media Items and Collections
	err = jobs.StartRefreshMediaItemsAndCollectionsJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Refresh Media Items and Collections cron job")
	}

	// Cronjob: Refresh Mediux Users
	err = jobs.StartRefreshMediuxUsersJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Refresh Mediux Users cron job")
	}

	// Cronjob: Check MediUX Site Link Availability
	err = jobs.StartCheckMediuxSiteLinkJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Check MediUX Site Link Availability cron job")
	}

	// Cronjob: Refresh AniDB Mappings (Plex-only; see the preload above)
	if config.Current.MediaServer.Type == "Plex" {
		err = jobs.StartRefreshAnidbMappingsJob()
		if err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Refresh AniDB Mappings cron job")
		}
	}

	// Cronjob: Start Check for Media Item Changes Job
	err = jobs.StartCheckForMediaItemChangesJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Check for Media Item Changes cron job")
	}

	// Cronjob: Start Handle Temp Ignored Items Job
	err = jobs.StartHandleTempIgnoredItemsJob()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to schedule Handle Temp Ignored Items cron job")
	}

	// Check MediUX Site Link Availability before starting the scheduler so the
	// immediate check cannot overlap its scheduled counterpart.
	config.AppLoadingStep = "Checking MediUX Site Link Availability"
	mediux.CheckSiteLinkAvailability()

	// Cron: Start Jobs Scheduler
	jobs.StartJobs()

	// Initialize MediUX WebSocket Listener
	//go autodownload.StartMediuxWebSocketClient()

	// Initialize Media Server WebSocket Listener (if supported)
	autodownload.StartOrRestartPlexWebSocketClient()

	success = true
	return success
}

// newAPIServer builds a listener with timeouts. ReadHeaderTimeout is the one that matters:
// without it a client can hold a connection open indefinitely by dribbling out headers.
// WriteTimeout is deliberately left unset, because image and log responses are proxied from
// a media server that can legitimately be slow.
func newAPIServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(dispatch),
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func startAPI() {
	startTLSIfConfigured()

	// Start HTTP Server
	logging.LOGGER.Info().Timestamp().Int("port", APP_PORT).
		Bool("full_routes", config.Loaded && config.Valid).
		Str("log_level", logging.LOGGER.GetLevel().String()).
		Msg("Starting HTTP Server")
	if err := newAPIServer(fmt.Sprintf(":%d", APP_PORT)).ListenAndServe(); err != nil {
		logging.LOGGER.Fatal().Err(err).Msg("Failed to start server")
	}
}

// startTLSIfConfigured starts an HTTPS listener on APP_TLS_PORT when
// TLS_CERT_FILE and TLS_KEY_FILE are set. It is an additional listener rather
// than a replacement: the UI's build-time /api rewrite targets
// http://localhost:8888, so the plain HTTP listener must stay up for
// UI-to-API traffic. Certificates are loaded once at startup; a restart is
// required to pick up rotated certs.
func startTLSIfConfigured() {
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")
	if certFile == "" && keyFile == "" {
		return
	}
	// Half-configured TLS is treated as fatal: silently serving HTTP only
	// would defeat the point of the user asking for HTTPS.
	if certFile == "" || keyFile == "" {
		logging.LOGGER.Fatal().Str("TLS_CERT_FILE", certFile).Str("TLS_KEY_FILE", keyFile).
			Msg("TLS_CERT_FILE and TLS_KEY_FILE must both be set to enable HTTPS")
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		logging.LOGGER.Fatal().Err(err).Str("TLS_CERT_FILE", certFile).Str("TLS_KEY_FILE", keyFile).
			Msg("Failed to load TLS certificate/key pair")
	}

	go func() {
		logging.LOGGER.Info().Timestamp().Int("port", APP_TLS_PORT).Msg("Starting HTTPS Server")
		if err := newAPIServer(fmt.Sprintf(":%d", APP_TLS_PORT)).ListenAndServeTLS(certFile, keyFile); err != nil {
			logging.LOGGER.Fatal().Err(err).Msg("Failed to start HTTPS server")
		}
	}()
}

// dispatch forwards to the currently active router.
func dispatch(w http.ResponseWriter, r *http.Request) {
	v := activeHandler.Load()
	h, ok := v.(http.Handler)
	if !ok || h == nil {
		// router not initialized yet (or stored value is wrong type)
		logging.LOGGER.Error().
			Timestamp().
			Str("path", r.URL.Path).
			Msg("activeHandler not initialized")
		http.Error(w, "Service starting up; router not initialized", http.StatusServiceUnavailable)
		return
	}

	h.ServeHTTP(w, r)
}
