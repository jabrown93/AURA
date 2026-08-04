package jobs

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/robfig/cron/v3"
)

// cronSpecCases mirrors the 5-field standard cron expressions hard-coded in
// this package's job-registration files (download_queue.go, download_auto.go,
// handle_temp_ignore_items.go, media_items_check_rating_key.go,
// media_items_refresh.go, mediux_check_link.go, mediux_refresh_users.go,
// anidb_refresh.go). Kometa's import cron is user-configured, not fixed, so
// it has no entry here.
//
// Every one of these is registered via gocron.CronJob(spec, false). Note that
// for well-formed 5-field specs this test cannot distinguish false from true:
// gocron's withSeconds knob selects robfig/cron's SecondOptional parser mode,
// which auto-detects field count and defaults a missing seconds field to 0
// either way (see normalizeFields in robfig/cron/v3), so a 5-field literal
// parses identically under both. false is still used everywhere for clarity
// and to match backend/config/validate.go#ValidateCron's cron.ParseStandard,
// which has no seconds concept at all. What this test does guard is that the
// next-run semantics of these specific literals match robfig/cron's standard
// parser exactly -- protecting against spec typos or accidental drift between
// what config validation accepts and what gocron actually schedules.
var cronSpecCases = []struct {
	name string
	spec string
}{
	{"Refresh AniDB Mappings Job", "0 4 * * 1"},
	{"AutoDownload Job (default)", "0 0 * * *"},
	{"Download Queue Processing Job", "* * * * *"},
	{"Handle Temp Ignored Items Job", "0 */1 * * *"},
	{"Check for Media Item Changes Job", "0 */6 * * *"},
	{"Refresh Media Items and Collections Job", "*/90 * * * *"},
	{"Check Mediux Site Link Availability Job", "*/60 * * * *"},
	{"Refresh Mediux Users Job", "*/90 * * * *"},
}

// TestCronSpecs_NextRunMatchesStandardCronSemantics schedules each spec with
// gocron.CronJob(spec, false) under synctest's fake clock, then independently
// computes the expected next run via robfig/cron's own ParseStandard (the
// same parser backend/config/validate.go#ValidateCron uses) and asserts the
// two agree. A mismatch here means the specs are no longer being interpreted
// as plain 5-field standard cron.
func TestCronSpecs_NextRunMatchesStandardCronSemantics(t *testing.T) {
	for _, tc := range cronSpecCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// The bubble's clock starts at 2000-01-01 00:00:00 UTC, whose
				// all-zero components would let day-of-week/hour bugs slip
				// through. Sleep forward to an instant with non-trivial
				// components (2000-01-02 10:30 UTC, a Sunday) so weekly/
				// hourly/every-N-minute schedules all exercise real
				// arithmetic. Inside the bubble this advances the fake clock
				// instantly.
				time.Sleep(34*time.Hour + 30*time.Minute)
				// .UTC() so robfig's Next computes in the same location the
				// scheduler is pinned to below; time.Now() reports the fake
				// clock in the local zone.
				reference := time.Now().UTC()

				sched, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
				if err != nil {
					t.Fatalf("failed to create scheduler: %v", err)
				}
				t.Cleanup(func() {
					if err := sched.Shutdown(); err != nil {
						t.Errorf("scheduler shutdown failed: %v", err)
					}
				})

				job, err := sched.NewJob(
					gocron.CronJob(tc.spec, false),
					gocron.NewTask(func() {}),
					gocron.WithName(tc.name),
				)
				if err != nil {
					t.Fatalf("failed to schedule job for spec %q: %v", tc.spec, err)
				}

				sched.Start()

				got, err := job.NextRun()
				if err != nil {
					t.Fatalf("NextRun() error: %v", err)
				}

				schedule, err := cron.ParseStandard(tc.spec)
				if err != nil {
					t.Fatalf("robfig/cron failed to parse spec %q: %v", tc.spec, err)
				}
				want := schedule.Next(reference)

				if !got.Equal(want) {
					t.Errorf("spec %q: gocron next run = %v, want %v (standard 5-field cron semantics per robfig/cron.ParseStandard)",
						tc.spec, got, want)
				}
			})
		})
	}
}
