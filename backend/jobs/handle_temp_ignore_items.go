package jobs

import (
	"aura/logging"
	"aura/mediaserver"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartHandleTempIgnoredItemsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if handleTempIgnoredItemsJobID != uuid.Nil {
		if err := sched.RemoveJob(handleTempIgnoredItemsJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Handle Temp Ignored Items Job")
		}
		handleTempIgnoredItemsJobID = uuid.Nil
		handleTempIgnoredItemsJob = nil
	}

	spec := "0 */1 * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled HandleTempIgnoredItemsJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Handle Temp Ignored Items", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			Err := mediaserver.HandleTempIgnoredItems(ctx)
			if Err.Message != "" {
				logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
					Str("next_run", formatNextRun(handleTempIgnoredItemsJob)).
					Msg("Error running Handle Temp Ignored Items Job")
			} else {
				logging.LOGGER.Info().Timestamp().
					Str("next_run", formatNextRun(handleTempIgnoredItemsJob)).
					Msg("Handle Temp Ignored Items Job Completed")
			}
			ld.Log()
		}),
		gocron.WithName("Handle Temp Ignored Items Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	handleTempIgnoredItemsJob = job
	handleTempIgnoredItemsJobID = job.ID()
	jobSpecs[handleTempIgnoredItemsJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "every 1 hour").
		Msg("Handle Temp Ignored Items Job Started")
	return nil
}

func RunHandleTempIgnoredItemsJobNow() {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return
	}

	if handleTempIgnoredItemsJobID == uuid.Nil || handleTempIgnoredItemsJob == nil {
		logging.LOGGER.Error().Timestamp().Msg("Handle Temp Ignored Items Job is not scheduled")
		return
	}

	go func() {
		if err := handleTempIgnoredItemsJob.RunNow(); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to run Handle Temp Ignored Items Job")
		}
	}()
}
