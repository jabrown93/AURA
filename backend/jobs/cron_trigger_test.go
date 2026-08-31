package jobs

import (
	"errors"
	"sync"
	"testing"
	"time"

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
			if err := TriggerJob(jobName, ""); err != nil && !errors.Is(err, ErrJobBusy) {
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

func TestTriggerJobReturnsBusyWithoutRecordingManualRun(t *testing.T) {
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

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	job, err := s.NewJob(
		gocron.CronJob("0 0 * * *", false),
		gocron.NewTask(func() {
			entered <- struct{}{}
			<-release
		}),
		gocron.WithName(jobName),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		t.Fatalf("failed to schedule job: %v", err)
	}
	s.Start()

	sched, downloadQueueJob, downloadQueueJobID = s, job, job.ID()
	manualPrevRun = map[uuid.UUID]string{}

	if err := job.RunNow(); err != nil {
		t.Fatalf("first RunNow() error: %v", err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first run never started")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		running, err := job.IsRunning()
		if err != nil {
			t.Fatalf("IsRunning() error: %v", err)
		}
		if running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first run never became observable as running")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := TriggerJob(jobName, job.ID().String()); !errors.Is(err, ErrJobBusy) {
		t.Fatalf("TriggerJob() error = %v, want ErrJobBusy", err)
	}
	if _, ok := manualPrevRun[job.ID()]; ok {
		t.Errorf("TriggerJob recorded a manual run for busy job %s", job.ID())
	}
}
