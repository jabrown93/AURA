package jobs

import (
	"aura/logging"
	"aura/mediux"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartRefreshMediuxUsersJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshMediuxUsersJobID != uuid.Nil {
		if err := sched.RemoveJob(refreshMediuxUsersJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Refresh Mediux Users Job")
		}
		refreshMediuxUsersJobID = uuid.Nil
		refreshMediuxUsersJob = nil
	}

	spec := "*/90 * * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled RefreshMediuxUsersJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Refresh Mediux Users", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			_, Err := mediux.GetAllUsers(ctx)
			if Err.Message != "" {
				logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
					Str("next_run", formatNextRun(refreshMediuxUsersJob)).
					Msg("Error running Refresh Mediux Users Job")
			} else {
				logging.LOGGER.Info().Timestamp().
					Str("next_run", formatNextRun(refreshMediuxUsersJob)).
					Msg("Refresh Mediux Users Job Completed")
			}
			ld.Log()
		}),
		gocron.WithName("Refresh Mediux Users Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	refreshMediuxUsersJob = job
	refreshMediuxUsersJobID = job.ID()
	jobSpecs[refreshMediuxUsersJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "every 90 minutes").
		Msg("Refresh Mediux Users Job Started")
	return nil
}
