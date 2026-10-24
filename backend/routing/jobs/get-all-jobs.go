package routes_jobs

import (
	"aura/jobs"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type GetAllJobs_Response struct {
	Jobs []jobs.JobInfo `json:"jobs"`
}

// GetAllJobs godoc
// @Summary      Get All Scheduled Jobs
// @Description  Retrieve a list of all scheduled jobs in the system, including their name, description, schedule, and next run time. This endpoint provides insight into the background tasks that are set up to run at specific intervals or times.
// @Tags         Jobs
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetAllJobs_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/jobs [get]
func GetAllJobs(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get All Scheduled Jobs", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetAllJobs_Response
	response.Jobs = jobs.GetListOfJobs()
	httpx.SendResponse(w, ld, response)
}
