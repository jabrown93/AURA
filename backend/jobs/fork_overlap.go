package jobs

import (
	"aura/logging"
	"errors"
	"sync"
)

// Fork: a job never overlaps with itself. A scheduled tick is skipped while the job is running
// and a manual trigger is refused with ErrJobBusy, so a manual run and a scheduled tick can't
// process the same work at once.

// ErrJobBusy is returned by TriggerJob when the job is already running.
var ErrJobBusy = errors.New("job is already running")

var (
	runningMu   sync.Mutex
	runningJobs = map[JobKey]bool{}
)

// claimJob marks key as running, returning false if it already was.
func claimJob(key JobKey) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	if runningJobs[key] {
		return false
	}
	runningJobs[key] = true
	return true
}

func releaseJob(key JobKey) {
	runningMu.Lock()
	defer runningMu.Unlock()
	delete(runningJobs, key)
}

// skipIfRunning wraps a scheduled job so a tick that fires while the job is running is dropped.
func skipIfRunning(key JobKey, job func()) func() {
	return func() {
		if !claimJob(key) {
			logging.LOGGER.Info().Timestamp().Str("job", string(key)).Msg("Job is still running, skipping this scheduled run")
			return
		}
		defer releaseJob(key)
		job()
	}
}
