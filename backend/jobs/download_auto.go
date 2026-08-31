package jobs

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/logging"
	"context"
	"runtime/debug"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartAutoDownloadJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if autodownloadJobID != uuid.Nil {
		if err := sched.RemoveJob(autodownloadJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing AutoDownload Job")
		}
		autodownloadJobID = uuid.Nil
		autodownloadJob = nil
	}

	enabled := config.Current.AutoDownload.Enabled
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("AutoDownload Job Stopped")
		return nil
	}

	spec := config.Current.AutoDownload.Cron
	if spec == "" {
		spec = "0 0 * * *" // Default to daily at midnight
	}

	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().
						Timestamp().
						Interface("recover", r).
						Str("stack", string(debug.Stack())).
						Msg("PANIC: in scheduled AutoDownload Job")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("AutoDownload Check", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			Err := autodownload.CheckAllItems(ctx)
			if Err.Message != "" {
				logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
					Str("next_run", formatNextRun(autodownloadJob)).
					Msg("Error running AutoDownload Job")
			} else {
				logging.LOGGER.Info().Timestamp().
					Str("next_run", formatNextRun(autodownloadJob)).
					Msg("AutoDownload Job Completed")
			}
		}),
		gocron.WithName("AutoDownload Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	autodownloadJob = job
	autodownloadJobID = job.ID()
	jobSpecs[autodownloadJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("AutoDownload Job Started")
	return nil
}
