package jobs

import (
	"aura/logging"
	"aura/mediux"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartCheckMediuxSiteLinkJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if checkMediuxSiteLinkJobID != uuid.Nil {
		if err := sched.RemoveJob(checkMediuxSiteLinkJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Check Mediux Site Link Availability Job")
		}
		checkMediuxSiteLinkJobID = uuid.Nil
		checkMediuxSiteLinkJob = nil
	}

	spec := "*/60 * * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled CheckMediuxSiteLinkJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Check Mediux Site Link Availability", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			mediux.CheckSiteLinkAvailability()
			ld.Log()
		}),
		gocron.WithName("Check Mediux Site Link Availability Job"),
	)
	if err != nil {
		return err
	}
	checkMediuxSiteLinkJob = job
	checkMediuxSiteLinkJobID = job.ID()
	jobSpecs[checkMediuxSiteLinkJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "every 60 minutes").
		Msg("Check Mediux Site Link Availability Job Started")
	return nil
}

func RunCheckMediuxSiteLinkJobNow() {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return
	}

	if checkMediuxSiteLinkJobID == uuid.Nil {
		logging.LOGGER.Error().Timestamp().Msg("Check Mediux Site Link Availability Job is not scheduled")
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in CheckMediuxSiteLinkJob")
			}
		}()
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Manual Job Run")
		action := ld.AddAction("Check Mediux Site Link Availability", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, action)
		mediux.CheckSiteLinkAvailability()
	}()
}
