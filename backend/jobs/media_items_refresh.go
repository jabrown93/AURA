package jobs

import (
	"aura/logging"
	"aura/mediaserver"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartRefreshMediaItemsAndCollectionsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshMediaItemsAndCollectionsJobID != uuid.Nil {
		if err := sched.RemoveJob(refreshMediaItemsAndCollectionsJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Refresh Media Items and Collections Job")
		}
		refreshMediaItemsAndCollectionsJobID = uuid.Nil
		refreshMediaItemsAndCollectionsJob = nil
	}

	spec := "*/90 * * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled RefreshMediaItemsAndCollectionsJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Refresh Media Items and Collections", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			mediaserver.GetAllLibrarySectionsAndItems(ctx, true)
			ld.Log()
		}),
		gocron.WithName("Refresh Media Items and Collections Job"),
	)
	if err != nil {
		return err
	}
	refreshMediaItemsAndCollectionsJob = job
	refreshMediaItemsAndCollectionsJobID = job.ID()
	jobSpecs[refreshMediaItemsAndCollectionsJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "every 90 minutes").
		Msg("Refresh Media Items and Collections Job Started")
	return nil
}
