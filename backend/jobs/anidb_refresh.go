package jobs

import (
	"aura/anidb"
	"aura/logging"
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartRefreshAnidbMappingsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshAnidbMappingsJobID != uuid.Nil {
		if err := sched.RemoveJob(refreshAnidbMappingsJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Refresh AniDB Mappings Job")
		}
		refreshAnidbMappingsJobID = uuid.Nil
		refreshAnidbMappingsJob = nil
	}

	// Weekly (Monday 04:00). The Fribb dataset changes slowly, so a frequent
	// refresh would be wasteful.
	spec := "0 4 * * 1"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled RefreshAnidbMappingsJob")
				}
			}()
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
			action := ld.AddAction("Refresh AniDB Mappings", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, action)
			anidb.PreloadAnidbMappings(ctx)
			ld.Log()
		}),
		gocron.WithName("Refresh AniDB Mappings Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	refreshAnidbMappingsJob = job
	refreshAnidbMappingsJobID = job.ID()
	jobSpecs[refreshAnidbMappingsJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Str("interval", "weekly").
		Msg("Refresh AniDB Mappings Job Started")
	return nil
}

func RunRefreshAnidbMappingsJobNow() {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return
	}

	if refreshAnidbMappingsJobID == uuid.Nil {
		logging.LOGGER.Error().Timestamp().Msg("Refresh AniDB Mappings Job is not scheduled")
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in RefreshAnidbMappingsJob")
			}
		}()
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Manual Job Run")
		action := ld.AddAction("Refresh AniDB Mappings", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, action)
		anidb.PreloadAnidbMappings(ctx)
	}()
}
