package main

import (
	"aura/database"
	"aura/jobs"
	"aura/logging"
	"context"
	"time"
)

// How long to wait for requests and running jobs to finish before stopping anyway
const shutdownTimeout = 8 * time.Second

// shutdown stops taking requests, waits for running jobs, then closes the database
func shutdown() {
	logging.LOGGER.Info().Timestamp().Msg("Shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := apiServer.Shutdown(ctx); err != nil {
		logging.LOGGER.Warn().Timestamp().Err(err).Msg("Stopped waiting for requests to finish")
	}
	shutdownTLS(ctx) // Fork
	jobs.StopJobs(ctx)
	jobs.StopDownloadQueueWorker(ctx)

	// Closing the database removes AURA.db-wal and AURA.db-shm
	logCtx, ld := logging.CreateLoggingContext(context.Background(), "Shutdown")
	logAction := ld.AddAction("Shutting Down", logging.LevelInfo)
	logCtx = logging.WithCurrentAction(logCtx, logAction)
	database.Close(logCtx)
	logAction.Complete()
	ld.Log()

	logging.LOGGER.Info().Timestamp().Msg("Shut down")
}
