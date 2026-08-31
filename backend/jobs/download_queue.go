package jobs

import (
	downloadqueue "aura/download/queue"
	"aura/logging"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func StartDownloadQueueJob() error {
	mu.Lock()
	defer mu.Unlock()

	if sched == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if downloadQueueJobID != uuid.Nil {
		if err := sched.RemoveJob(downloadQueueJobID); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to remove existing Download Queue Processing Job")
		}
		delete(jobSpecs, downloadQueueJobID)
		downloadQueueJobID = uuid.Nil
		downloadQueueJob = nil
	}

	spec := "* * * * *"
	job, err := sched.NewJob(
		gocron.CronJob(spec, false),
		gocron.NewTask(func() {
			defer func() {
				if r := recover(); r != nil {
					logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled Download Queue Job")
				}
			}()
			downloadqueue.ProcessQueueItems()
			downloadqueue.ProcessCollectionQueueItems()
		}),
		gocron.WithName("Download Queue Processing Job"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	downloadQueueJob = job
	downloadQueueJobID = job.ID()
	jobSpecs[downloadQueueJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", "* * * * *").
		Str("interval", "every minute").
		Msg("Download Queue Processing Job Started")

	return nil
}
