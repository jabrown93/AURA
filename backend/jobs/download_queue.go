package jobs

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
)

func StartDownloadQueueJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if downloadQueueJobID != 0 {
		c.Remove(downloadQueueJobID)
		delete(jobSpecs, downloadQueueJobID)
		downloadQueueJobID = 0
	}

	spec := "* * * * *"
	var err error
	downloadQueueJobID, err = c.AddFunc(spec, func() {
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled Download Queue Job")
			}
		}()
		downloadqueue.ProcessQueueItems()
		downloadqueue.ProcessCollectionQueueItems()
	})
	if err != nil {
		return err
	}
	jobSpecs[downloadQueueJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", "* * * * *").
		Str("interval", "every minute").
		Msg("Download Queue Processing Job Started")

	return nil
}
