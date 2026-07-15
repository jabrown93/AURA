package jobs

import (
	"aura/logging"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	c  *cron.Cron
	mu sync.Mutex

	jobSpecs = map[cron.EntryID]string{}

	// Runs Always
	downloadQueueJobID                   cron.EntryID = 0
	refreshMediaItemsAndCollectionsJobID cron.EntryID = 0
	refreshMediuxUsersJobID              cron.EntryID = 0
	checkMediuxSiteLinkJobID             cron.EntryID = 0
	checkForMediaItemChangesJobID        cron.EntryID = 0
	handleTempIgnoredItemsJobID          cron.EntryID = 0
	refreshAnidbMappingsJobID            cron.EntryID = 0

	// Configurable
	autodownloadJobID cron.EntryID = 0
	kometaImportJobID cron.EntryID = 0
)

var manualPrevRun = map[cron.EntryID]string{}

func init() {
	c = cron.New()
}

func StartJobs() {
	if c != nil {
		c.Start()
		logging.LOGGER.Info().Timestamp().Msg("Cron Jobs Scheduler Started")
	}
}

type JobInfo struct {
	ID      cron.EntryID `json:"id"`
	Spec    string       `json:"spec"`
	NextRun string       `json:"next_run"`
	PrevRun string       `json:"prev_run"`
	JobName string       `json:"job_name"`
}

func GetListOfJobs() []JobInfo {
	mu.Lock()
	defer mu.Unlock()

	var jobs []JobInfo
	if c != nil {
		entries := c.Entries()
		for _, entry := range entries {
			prevRun := entry.Prev.String()
			if manual, ok := manualPrevRun[entry.ID]; ok {
				prevRun = manual
			} else if prevRun == "0001-01-01 00:00:00 +0000 UTC" {
				prevRun = ""
			} else {
				prevRun = entry.Prev.Format("2006-01-02 15:04:05")
			}

			jobInfo := JobInfo{
				ID:      entry.ID,
				Spec:    "",
				NextRun: entry.Next.Format("2006-01-02 15:04:05"),
				PrevRun: prevRun,
				JobName: "",
			}

			// Use stored spec; cron doesn't expose it from the parsed schedule.
			if spec, ok := jobSpecs[entry.ID]; ok {
				jobInfo.Spec = spec
			} else {
				// optional fallback: at least show concrete schedule type
				jobInfo.Spec = fmt.Sprintf("%T", entry.Schedule)
			}

			switch entry.ID {
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

	var entryID cron.EntryID
	switch jobName {
	case "Download Queue Processing Job":
		entryID = downloadQueueJobID
	case "AutoDownload Job":
		entryID = autodownloadJobID
	case "Refresh Media Items and Collections Job":
		entryID = refreshMediaItemsAndCollectionsJobID
	case "Refresh Mediux Users Job":
		entryID = refreshMediuxUsersJobID
	case "Check Mediux Site Link Availability Job":
		entryID = checkMediuxSiteLinkJobID
	case "Check for Media Item Changes Job":
		entryID = checkForMediaItemChangesJobID
	case "Handle Temp Ignored Items Job":
		entryID = handleTempIgnoredItemsJobID
	case "Refresh AniDB Mappings Job":
		entryID = refreshAnidbMappingsJobID
	case "Kometa Asset Import Job":
		entryID = kometaImportJobID
	default:
		return fmt.Errorf("unknown job name: %s", jobName)
	}

	if entryID == 0 {
		return fmt.Errorf("job not found or not scheduled: %s", jobName)
	}

	entry := c.Entry(entryID)
	if entry.ID == 0 {
		return fmt.Errorf("job entry not found for ID: %d", entryID)
	}

	go func() {
		if entry.Job != nil {
			manualPrevRun[entry.ID] = time.Now().Format("2006-01-02 15:04:05")
			entry.Job.Run()
		}
	}()

	return nil
}
