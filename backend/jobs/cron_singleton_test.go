package jobs

import (
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// TestSingletonModeBlocksOverlappingRunNow pins the scheduler-level backstop.
// Every sched.NewJob call in this package passes
// gocron.WithSingletonMode(gocron.LimitModeReschedule), so duplicate scheduler
// submissions remain serialized in addition to the application-level runner.
//
// The first run is held hostage inside the task body, then a second RunNow is
// fired. Under reschedule mode the second run is dropped and can never enter
// however long we wait; the withoutSingletonMode control proves the harness
// really does observe an overlapping run, so the first case cannot pass
// vacuously.
func TestSingletonModeBlocksOverlappingRunNow(t *testing.T) {
	tests := []struct {
		name        string
		opts        []gocron.JobOption
		wantOverlap bool
	}{
		{
			name:        "withSingletonMode",
			opts:        []gocron.JobOption{gocron.WithSingletonMode(gocron.LimitModeReschedule)},
			wantOverlap: false,
		},
		{
			name:        "withoutSingletonMode",
			wantOverlap: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := gocron.NewScheduler()
			if err != nil {
				t.Fatalf("failed to create scheduler: %v", err)
			}
			// Registered first so it runs last: the deferred close(release)
			// below must unblock the in-flight task before Shutdown waits on it.
			t.Cleanup(func() {
				if err := s.Shutdown(); err != nil {
					t.Errorf("scheduler shutdown failed: %v", err)
				}
			})

			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			defer close(release)

			job, err := s.NewJob(
				gocron.CronJob("0 0 * * *", false),
				gocron.NewTask(func() {
					entered <- struct{}{}
					<-release
				}),
				append([]gocron.JobOption{gocron.WithName(tc.name)}, tc.opts...)...,
			)
			if err != nil {
				t.Fatalf("failed to schedule job: %v", err)
			}
			s.Start()

			if err := job.RunNow(); err != nil {
				t.Fatalf("first RunNow() error: %v", err)
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("first run never started")
			}

			if err := job.RunNow(); err != nil {
				t.Fatalf("second RunNow() error: %v", err)
			}
			var gotOverlap bool
			select {
			case <-entered:
				gotOverlap = true
			case <-time.After(500 * time.Millisecond):
			}

			if gotOverlap != tc.wantOverlap {
				t.Errorf("second run overlapped in-flight run = %v, want %v", gotOverlap, tc.wantOverlap)
			}
		})
	}
}
