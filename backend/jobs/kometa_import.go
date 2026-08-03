package jobs

import (
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"runtime/debug"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// StartKometaImportJob (re)schedules the periodic Kometa asset import job. It is a no-op
// unless Kometa mode is enabled and an ImportCron expression is configured; the import can
// still be run on demand via the API regardless of this schedule.
func StartKometaImportJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if kometaImportJobID != uuid.Nil {
		if err := sched.RemoveJob(kometaImportJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Kometa Asset Import Job")
		}
		kometaImportJobID = uuid.Nil
		kometaImportJob = nil
	}

	k := config.Current.Images.Kometa
	if !k.Enabled || k.ImportCron == "" {
		logging.LOGGER.Info().Timestamp().Msg("Kometa Asset Import Job Stopped")
		return nil
	}

	spec := k.ImportCron

	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().
						Timestamp().
						Interface("recover", r).
						Str("stack", string(debug.Stack())).
						Msg("PANIC: in scheduled Kometa Asset Import Job")
				}
			}()
			if started := kometa.StartImport(); !started {
				logging.LOGGER.Warn().Timestamp().Msg("Kometa Asset Import skipped (already running or not enabled)")
			}
		}),
		gocron.WithName("Kometa Asset Import Job"),
	)
	if err != nil {
		return err
	}
	kometaImportJob = job
	kometaImportJobID = job.ID()
	jobSpecs[kometaImportJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Kometa Asset Import Job Started")
	return nil
}
