# Job Busy Response Design

## Goal

Prevent manual job triggers from reporting success when singleton mode drops them because that job is already running.

## Behavior

- Each job name owns one persistent runner with an atomic busy flag and the current task closure.
- Scheduled ticks and manual triggers both enter through that runner.
- Manual triggers atomically reserve the runner before launching the task directly; a failed reservation returns `ErrJobBusy`.
- Busy triggers do not update `manualPrevRun`.
- Accepted manual triggers record `manualPrevRun` after reserving the runner.
- Runners survive scheduler job replacement, so the same gate protects old and new job generations.
- gocron singleton mode remains as a secondary scheduler-level guard.
- `/api/jobs` maps `ErrJobBusy` to HTTP `409 Conflict` using the existing JSON error shape.
- Other trigger errors remain HTTP `500`.

## Scope

Apply the runner to all nine scheduled jobs and remove the two remaining zero-caller `Run*Now` helpers that bypass the API contract. No queueing, retry policy, scheduler replacement, frontend changes, or new dependency.

## Verification

- Runner-level tests prove scheduled/manual admission is mutually exclusive and a runner remains shared when its task closure is replaced.
- Trigger-level test holds one run open and verifies another trigger returns `ErrJobBusy` without changing `manualPrevRun`.
- Route-level test verifies busy maps to HTTP 409.
- Existing backend tests, formatting, and build remain clean.
