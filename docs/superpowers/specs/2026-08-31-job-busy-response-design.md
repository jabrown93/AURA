# Job Busy Response Design

## Goal

Prevent manual job triggers from reporting success when singleton mode drops them because that job is already running.

## Behavior

- `jobs.TriggerJob` checks `gocron.Job.IsRunning()` before requesting `RunNow()`.
- Running jobs return a sentinel `ErrJobBusy`.
- Busy triggers do not update `manualPrevRun`.
- Successful triggers call `RunNow()` synchronously, then record `manualPrevRun`.
- `/api/jobs` maps `ErrJobBusy` to HTTP `409 Conflict` using the existing JSON error shape.
- Other trigger errors remain HTTP `500`.

## Scope

No queueing, retry policy, scheduler replacement, frontend changes, or new dependency.

## Verification

- Job-level test holds one singleton run open and verifies another trigger returns `ErrJobBusy` without changing `manualPrevRun`.
- Route-level test verifies busy maps to HTTP 409.
- Existing backend tests, formatting, and build remain clean.

## Known Limit

`IsRunning()` observes executions after gocron records their start. It closes the reported active-run defect but does not promise atomic admission against a scheduler tick accepted in the same instant.
