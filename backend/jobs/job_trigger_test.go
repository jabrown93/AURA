package jobs

import (
	"errors"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

func TestTriggerJobReturnsBusyWithoutRecordingManualRun(t *testing.T) {
	const jobName = "Download Queue Processing Job"

	prevSched, prevJob, prevID, prevRuns, prevRunners := sched, downloadQueueJob, downloadQueueJobID, manualPrevRun, jobRunners
	t.Cleanup(func() {
		sched, downloadQueueJob, downloadQueueJobID, manualPrevRun, jobRunners = prevSched, prevJob, prevID, prevRuns, prevRunners
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
	jobRunners = map[string]*jobRunner{}
	runner := configureJobRunner(jobName, func() {
		entered <- struct{}{}
		<-release
	})
	go runner.runScheduled()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduled run never started")
	}

	if err := TriggerJob(jobName, job.ID().String()); !errors.Is(err, ErrJobBusy) {
		t.Fatalf("TriggerJob() error = %v, want ErrJobBusy", err)
	}
	if _, ok := manualPrevRun[job.ID()]; ok {
		t.Errorf("TriggerJob recorded a manual run for busy job %s", job.ID())
	}
}
