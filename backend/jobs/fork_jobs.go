package jobs

import (
	"aura/anidb"
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"context"
	"fmt"
)

// Fork: jobs that only exist in the fork, registered alongside upstream's.

const (
	JobKeyKometaImport JobKey = "kometa_import"
	JobKeyAnidbRefresh JobKey = "anidb_refresh"
)

func init() {
	jobDefinitions = append(jobDefinitions,
		jobDefinition{
			Key:         JobKeyKometaImport,
			Name:        "Kometa Asset Import Job",
			Description: "Imports images already in the Kometa asset directory as saved sets. Only runs when Kometa mode is on.",
			Setting:     func() config.Config_JobSetting { return config.Current.Jobs.KometaImport },
			Default:     config.ForkJobDefaults.KometaImport,
			Start: func() error {
				return startForkJob(JobKeyKometaImport, config.Current.Jobs.KometaImport, config.ForkJobDefaults.KometaImport, runKometaImport)
			},
			RunNow: func() { runForkJobNow(JobKeyKometaImport, runKometaImport) },
		},
		jobDefinition{
			Key:         JobKeyAnidbRefresh,
			Name:        "Refresh AniDB Mappings Job",
			Description: "Refreshes the AniDB to TMDB mapping used to match anime items from the HAMA agent.",
			Setting:     func() config.Config_JobSetting { return config.Current.Jobs.AnidbRefresh },
			Default:     config.ForkJobDefaults.AnidbRefresh,
			Start: func() error {
				return startForkJob(JobKeyAnidbRefresh, config.Current.Jobs.AnidbRefresh, config.ForkJobDefaults.AnidbRefresh, runAnidbRefresh)
			},
			RunNow: func() { runForkJobNow(JobKeyAnidbRefresh, runAnidbRefresh) },
		},
	)
}

func runKometaImport(context.Context) {
	if !config.Current.Images.Kometa.Enabled {
		return
	}
	if started := kometa.RunImport(); !started {
		logging.LOGGER.Warn().Timestamp().Msg("Kometa Asset Import skipped (already running or not enabled)")
	}
}

func runAnidbRefresh(ctx context.Context) {
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Refresh AniDB Mappings", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	anidb.PreloadAnidbMappings(ctx)
	ld.Log()
}

func startForkJob(key JobKey, setting config.Config_JobSetting, def config.JobSettingDefault, body func(context.Context)) error {
	mu.Lock()
	defer mu.Unlock()
	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}
	removeScheduledJob(key)
	enabled, spec := setting.Resolve(def)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Str("job", string(key)).Msg("Job Stopped")
		return nil
	}
	if err := addScheduledJob(key, spec, func() {
		defer logging.RecoverPanic(string(key))
		body(context.Background())
	}); err != nil {
		return fmt.Errorf("failed to schedule %s: %w", key, err)
	}
	logging.LOGGER.Info().Timestamp().Str("job", string(key)).Str("cron", spec).Msg("Job Started")
	return nil
}

func runForkJobNow(key JobKey, body func(context.Context)) {
	go func() {
		defer releaseJob(key) // pairs with claimJob in TriggerJob
		defer logging.RecoverPanic(string(key))
		recordManualRun(key)
		body(context.Background())
	}()
}
