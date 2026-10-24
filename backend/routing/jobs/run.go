package routes_jobs

import (
	"aura/jobs"
	"aura/logging"
	"aura/utils/httpx"
	"errors"
	"fmt"
	"net/http"
)

type runJobResponse struct {
	Message string `json:"message"`
}

// RunJob godoc
// @Summary      Run Job
// @Description  Trigger a specific job to run immediately by providing the job's ID (key) as a query parameter. This endpoint allows for manual execution of a job outside of its regular schedule - including jobs currently disabled from automatic scheduling - which can be useful for testing or urgent tasks.
// @Tags         Jobs
// @Accept       json
// @Produce      json
// @Param        job_id  query     string  true  "ID of the Job to Run"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200       {object}  httpx.JSONResponse{data=runJobResponse}
// @Failure      409       {object}  httpx.JSONResponse "Job is already running"
// @Failure      500       {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/jobs [post]
func RunJob(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Run Job", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response runJobResponse

	actionGetQueryParams := ld.AddAction("Get all query params", logging.LevelTrace)
	// Get the Job ID (key) from the URL parameters
	jobID := r.URL.Query().Get("job_id")

	// Validate the Job ID
	if jobID == "" {
		actionGetQueryParams.SetError("Missing Query Parameters", "The job_id query parameter is required",
			map[string]any{
				"job_id": jobID,
			})
		httpx.SendResponse(w, ld, response)
		return
	}
	actionGetQueryParams.Complete()

	// Trigger the Job
	actionTriggerJob := ld.AddAction("Trigger Job", logging.LevelInfo)
	err := jobs.TriggerJob(jobs.JobKey(jobID))
	// Fork: a job that is already running is refused with 409 instead of overlapping itself.
	if errors.Is(err, jobs.ErrJobBusy) {
		actionTriggerJob.SetError("Job is already running", "Wait for the current run to finish, then try again",
			map[string]any{"job_id": jobID})
		httpx.SendResponseWithStatus(w, ld, response, http.StatusConflict)
		return
	}
	if err != nil {
		actionTriggerJob.SetError("Failed to Trigger Job", "An error occurred while trying to trigger the job",
			map[string]any{
				"error":  err.Error(),
				"job_id": jobID,
			})
		httpx.SendResponse(w, ld, response)
		return
	}
	actionTriggerJob.Complete()

	response.Message = fmt.Sprintf("Job '%s' has been triggered successfully", jobID)
	httpx.SendResponse(w, ld, response)
}
