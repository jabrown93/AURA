package jobs

import (
	"aura/logging"
	"fmt"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

var (
	sched gocron.Scheduler
	mu    sync.Mutex

	jobSpecs = map[uuid.UUID]string{}

	// Runs Always
	downloadQueueJobID                   uuid.UUID = uuid.Nil
	refreshMediaItemsAndCollectionsJobID uuid.UUID = uuid.Nil
	refreshMediuxUsersJobID              uuid.UUID = uuid.Nil
	checkMediuxSiteLinkJobID             uuid.UUID = uuid.Nil
	checkForMediaItemChangesJobID        uuid.UUID = uuid.Nil
	handleTempIgnoredItemsJobID          uuid.UUID = uuid.Nil
	refreshAnidbMappingsJobID            uuid.UUID = uuid.Nil

	// Configurable
	autodownloadJobID uuid.UUID = uuid.Nil
	kometaImportJobID uuid.UUID = uuid.Nil

	// Job handles, kept alongside the ID vars above so TriggerJob and the
	// per-job "run now" helpers can call RunNow() without a lookup accessor;
	// gocron v2 doesn't expose one.
	downloadQueueJob                   gocron.Job
	refreshMediaItemsAndCollectionsJob gocron.Job
	refreshMediuxUsersJob              gocron.Job
	checkMediuxSiteLinkJob             gocron.Job
	checkForMediaItemChangesJob        gocron.Job
	handleTempIgnoredItemsJob          gocron.Job
	refreshAnidbMappingsJob            gocron.Job
	autodownloadJob                    gocron.Job
	kometaImportJob                    gocron.Job
)

var manualPrevRun = map[uuid.UUID]string{}

func init() {
	var err error
	sched, err = gocron.NewScheduler()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to initialize Cron Jobs Scheduler")
		sched = nil
	}
}

func StartJobs() {
	if sched != nil {
		sched.Start()
		logging.LOGGER.Info().Timestamp().Msg("Cron Jobs Scheduler Started")
	}
}

type JobInfo struct {
	ID      uuid.UUID `json:"id"`
	Spec    string    `json:"spec"`
	NextRun string    `json:"next_run"`
	PrevRun string    `json:"prev_run"`
	JobName string    `json:"job_name"`
}

// formatNextRun resolves a job's next scheduled run time for display/logging.
// Returns an empty string (and logs) if the job is nil or gocron can't compute it.
func formatNextRun(job gocron.Job) string {
	if job == nil {
		return ""
	}
	next, err := job.NextRun()
	if err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Str("job_id", job.ID().String()).
			Msg("Failed to get job next run time")
		return ""
	}
	return next.Format("2006-01-02 15:04:05")
}

func GetListOfJobs() []JobInfo {
	mu.Lock()
	defer mu.Unlock()

	var jobs []JobInfo
	if sched != nil {
		entries := sched.Jobs()
		for _, entry := range entries {
			entryID := entry.ID()

			prevRun := ""
			if manual, ok := manualPrevRun[entryID]; ok {
				prevRun = manual
			} else if last, err := entry.LastRunStartedAt(); err != nil {
				logging.LOGGER.Error().Timestamp().Err(err).Str("job_id", entryID.String()).
					Msg("Failed to get job last run start time")
			} else if !last.IsZero() {
				prevRun = last.Format("2006-01-02 15:04:05")
			}

			nextRun := ""
			if next, err := entry.NextRun(); err != nil {
				logging.LOGGER.Error().Timestamp().Err(err).Str("job_id", entryID.String()).
					Msg("Failed to get job next run time")
			} else {
				nextRun = next.Format("2006-01-02 15:04:05")
			}

			jobInfo := JobInfo{
				ID:      entryID,
				Spec:    "",
				NextRun: nextRun,
				PrevRun: prevRun,
				JobName: "",
			}

			// Use stored spec; gocron doesn't expose the original cron string from the job.
			if spec, ok := jobSpecs[entryID]; ok {
				jobInfo.Spec = spec
			} else {
				jobInfo.Spec = entry.Name()
			}

			switch entryID {
			case downloadQueueJobID:
				jobInfo.JobName = "Download Queue Processing Job"
			case autodownloadJobID:
				jobInfo.JobName = "AutoDownload Job"
			case refreshMediaItemsAndCollectionsJobID:
				jobInfo.JobName = "Refresh Media Items and Collections Job"
			case refreshMediuxUsersJobID:
				jobInfo.JobName = "Refresh Mediux Users Job"
			case checkMediuxSiteLinkJobID:
				jobInfo.JobName = "Check Mediux Site Link Availability Job"
			case checkForMediaItemChangesJobID:
				jobInfo.JobName = "Check for Media Item Changes Job"
			case handleTempIgnoredItemsJobID:
				jobInfo.JobName = "Handle Temp Ignored Items Job"
			case refreshAnidbMappingsJobID:
				jobInfo.JobName = "Refresh AniDB Mappings Job"
			case kometaImportJobID:
				jobInfo.JobName = "Kometa Asset Import Job"
			default:
				jobInfo.JobName = "Unknown Job"
			}
			jobs = append(jobs, jobInfo)
		}
	}
	return jobs
}

func TriggerJob(jobName string, jobID string) error {
	mu.Lock()
	defer mu.Unlock()

	var entryID uuid.UUID
	var job gocron.Job
	switch jobName {
	case "Download Queue Processing Job":
		entryID = downloadQueueJobID
		job = downloadQueueJob
	case "AutoDownload Job":
		entryID = autodownloadJobID
		job = autodownloadJob
	case "Refresh Media Items and Collections Job":
		entryID = refreshMediaItemsAndCollectionsJobID
		job = refreshMediaItemsAndCollectionsJob
	case "Refresh Mediux Users Job":
		entryID = refreshMediuxUsersJobID
		job = refreshMediuxUsersJob
	case "Check Mediux Site Link Availability Job":
		entryID = checkMediuxSiteLinkJobID
		job = checkMediuxSiteLinkJob
	case "Check for Media Item Changes Job":
		entryID = checkForMediaItemChangesJobID
		job = checkForMediaItemChangesJob
	case "Handle Temp Ignored Items Job":
		entryID = handleTempIgnoredItemsJobID
		job = handleTempIgnoredItemsJob
	case "Refresh AniDB Mappings Job":
		entryID = refreshAnidbMappingsJobID
		job = refreshAnidbMappingsJob
	case "Kometa Asset Import Job":
		entryID = kometaImportJobID
		job = kometaImportJob
	default:
		return fmt.Errorf("unknown job name: %s", jobName)
	}

	if entryID == uuid.Nil || job == nil {
		return fmt.Errorf("job not found or not scheduled: %s", jobName)
	}

	go func() {
		manualPrevRun[entryID] = time.Now().Format("2006-01-02 15:04:05")
		if err := job.RunNow(); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Str("job_name", jobName).
				Msg("Failed to trigger job")
		}
	}()

	return nil
}
