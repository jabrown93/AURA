package jobs

import (
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"runtime/debug"
)

// StartKometaImportJob (re)schedules the periodic Kometa asset import job. It is a no-op
// unless Kometa mode is enabled and an ImportCron expression is configured; the import can
// still be run on demand via the API regardless of this schedule.
func StartKometaImportJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if kometaImportJobID != 0 {
		c.Remove(kometaImportJobID)
		kometaImportJobID = 0
	}

	k := config.Current.Images.Kometa
	if !k.Enabled || k.ImportCron == "" {
		logging.LOGGER.Info().Timestamp().Msg("Kometa Asset Import Job Stopped")
		return nil
	}

	spec := k.ImportCron

	var err error
	kometaImportJobID, err = c.AddFunc(spec, func() {
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
	})
	if err != nil {
		return err
	}
	jobSpecs[kometaImportJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Kometa Asset Import Job Started")
	return nil
}
