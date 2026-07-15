package jobs

import (
	"aura/anidb"
	"aura/logging"
	"context"
)

func StartRefreshAnidbMappingsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshAnidbMappingsJobID != 0 {
		c.Remove(refreshAnidbMappingsJobID)
		refreshAnidbMappingsJobID = 0
	}

	var err error
	// Weekly (Monday 04:00). The Fribb dataset changes slowly, so a frequent
	// refresh would be wasteful.
	spec := "0 4 * * 1"
	refreshAnidbMappingsJobID, err = c.AddFunc(spec, func() {
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
	})
	if err != nil {
		return err
	}
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

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return
	}

	if refreshAnidbMappingsJobID == 0 {
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
