package jobs

import (
	"aura/logging"
	"aura/mediaserver"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartCheckForMediaItemChangesJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if checkForMediaItemChangesJobID != uuid.Nil {
		if err := sched.RemoveJob(checkForMediaItemChangesJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Check for Media Item Changes Job")
		}
		checkForMediaItemChangesJobID = uuid.Nil
		checkForMediaItemChangesJob = nil
	}

	spec := "0 */6 * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(configureJobRunner("Check for Media Item Changes Job", func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled CheckForMediaItemChangesJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Check for Media Item Changes", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			Err := mediaserver.CheckForMediaItemChanges(ctx)
			if Err.Message != "" {
				logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
					Str("next_run", formatNextRun(checkForMediaItemChangesJob)).
					Msg("Error running Check for Media Item Changes Job")
			} else {
				logging.LOGGER.Info().Timestamp().
					Str("next_run", formatNextRun(checkForMediaItemChangesJob)).
					Msg("Check for Media Item Changes Job Completed")
			}
			ld.Log()
		}).runScheduled),
		gocron.WithName("Check for Media Item Changes Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	checkForMediaItemChangesJob = job
	checkForMediaItemChangesJobID = job.ID()
	jobSpecs[checkForMediaItemChangesJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "every 6 hours").
		Msg("Check for Media Item Changes Job Started")
	return nil
}
