package main

import (
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
	"fmt"
	"net/http"
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
		return success
	}
	if config.Current.MediaServer.Type == "Jellyfin" || config.Current.MediaServer.Type == "Emby" {
		// Get Admin User for Emby/Jellyfin
		config.AppLoadingStep = "Retrieving Media Server Admin User"
		ejUserID, initErr := mediaserver.GetAdminUser(ctx, &config.Current.MediaServer)
		if initErr.Message != "" {
			config.MediaServerValid = false
			return success
		} else if ejUserID == "" {
			config.MediaServerValid = false
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

	// Validate MediUX Token
	config.AppLoadingStep = "Validating MediUX Token"
	mediuxTokenValid, mediuxErr := mediux.ValidateToken(ctx, config.Current.Mediux.ApiToken)
	if mediuxErr.Message != "" || !mediuxTokenValid {
		config.MediuxValid = false
		return success
	}

	if config.MediaServerValid || config.MediuxValid {
		success = true
	}

	return success
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
		migrationsCompleted, _ := migration.RunMigrations()
		logging.LOGGER.Info().Timestamp().Msgf("%d database migrations performed", migrationsCompleted)
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
		downloadqueue.LatestInfo.Time = time.Now()
		downloadqueue.LatestInfo.Status = downloadqueue.LAST_STATUS_ERROR
		downloadqueue.LatestInfo.Message = "Failed to schedule Download Queue Processing"
		downloadqueue.LatestInfo.Errors = []string{err.Error()}
		downloadqueue.LatestInfo.Warnings = []string{}
	} else {
		downloadqueue.LatestInfo.Time = time.Now()
		downloadqueue.LatestInfo.Status = downloadqueue.LAST_STATUS_IDLE
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

	// Cron: Start Jobs Scheduler
	jobs.StartJobs()

	// Check MediUX Site Link Availability immediately on startup
	config.AppLoadingStep = "Checking MediUX Site Link Availability"
	mediux.CheckSiteLinkAvailability()

	// Initialize MediUX WebSocket Listener
	//go autodownload.StartMediuxWebSocketClient()

	// Initialize Media Server WebSocket Listener (if supported)
	autodownload.StartOrRestartPlexWebSocketClient()

	success = true
	return success
}

func startAPI() {
	// Start HTTP Server
	logging.LOGGER.Info().Timestamp().Int("port", APP_PORT).
		Bool("full_routes", config.Loaded && config.Valid).
		Str("log_level", logging.LOGGER.GetLevel().String()).
		Msg("Starting HTTP Server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", APP_PORT), http.HandlerFunc(dispatch)); err != nil {
		logging.LOGGER.Fatal().Err(err).Msg("Failed to start server")
	}
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
