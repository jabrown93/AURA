package jobs

import (
	"sync"
	"testing"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

// TestTriggerJob_ManualPrevRunGuardedAgainstGetListOfJobs pins the manualPrevRun
// map access pattern: GetListOfJobs reads it under mu on every /api/jobs poll
// while TriggerJob records a manual run. A concurrent map read/write is an
// unrecoverable Go runtime fatal, so this must stay inside the lock. Run with
// -race to see an unguarded write reported as a data race rather than as a
// nondeterministic process kill.
func TestTriggerJob_ManualPrevRunGuardedAgainstGetListOfJobs(t *testing.T) {
	const jobName = "Download Queue Processing Job"

	prevSched, prevJob, prevID, prevRuns := sched, downloadQueueJob, downloadQueueJobID, manualPrevRun
	t.Cleanup(func() {
		sched, downloadQueueJob, downloadQueueJobID, manualPrevRun = prevSched, prevJob, prevID, prevRuns
	})

	s, err := gocron.NewScheduler()
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Shutdown(); err != nil {
			t.Errorf("scheduler shutdown failed: %v", err)
		}
	})

	job, err := s.NewJob(
		gocron.CronJob("0 0 * * *", false),
		gocron.NewTask(func() {}),
		gocron.WithName(jobName),
	)
	if err != nil {
		t.Fatalf("failed to schedule job: %v", err)
	}
	s.Start()

	sched, downloadQueueJob, downloadQueueJobID = s, job, job.ID()
	manualPrevRun = map[uuid.UUID]string{}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := TriggerJob(jobName, ""); err != nil {
				t.Errorf("TriggerJob() error: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			GetListOfJobs()
		}()
	}
	wg.Wait()

	if _, ok := manualPrevRun[job.ID()]; !ok {
		t.Errorf("TriggerJob did not record a manual previous run for job %s", job.ID())
	}
}
