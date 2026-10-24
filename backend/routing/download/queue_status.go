package routes_download

import (
	"aura/database"
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
	"time"
)

type GetDownloadQueueStatus_Response struct {
	Time     time.Time            `json:"time"`
	Status   downloadqueue.Status `json:"status"`
	Message  string               `json:"message"`
	Warnings []string             `json:"warnings"`
	Errors   []string             `json:"errors"`
}

// GetDownloadQueueStatus godoc
// @Summary      Download Queue - Get Status
// @Description  Retrieve the current status of the download queue, including the latest status message, any warnings or errors, and the timestamp of the last update. This endpoint provides insight into the overall health and activity of the download queue.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetDownloadQueueStatus_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue [get]
func GetDownloadQueueStatus(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Get Status", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var response GetDownloadQueueStatus_Response
	response.Status = downloadqueue.LAST_STATUS_IDLE

	jobs, Err := database.GetAllDownloadQueueJobs(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if len(jobs) == 0 {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Prefer a currently-processing job, otherwise report on the most recently finished one.
	var current *database.DownloadQueueJob
	var latestFinished *database.DownloadQueueJob
	for i := range jobs {
		job := &jobs[i]
		if job.Status == "processing" {
			current = job
			break
		}
		if job.FinishedAt != nil && (latestFinished == nil || job.FinishedAt.After(*latestFinished.FinishedAt)) {
			latestFinished = job
		}
	}

	switch {
	case current != nil:
		response.Time = current.CreatedAt
		response.Status = downloadqueue.LAST_STATUS_PROCESSING
		response.Message = current.MediaItemTitle
	case latestFinished != nil:
		response.Time = *latestFinished.FinishedAt
		response.Message = latestFinished.ResultMessage
		response.Warnings = latestFinished.ResultWarnings
		response.Errors = latestFinished.ResultErrors
		switch latestFinished.Status {
		case "warning":
			response.Status = downloadqueue.LAST_STATUS_WARNING
		case "error":
			response.Status = downloadqueue.LAST_STATUS_ERROR
		default:
			response.Status = downloadqueue.LAST_STATUS_SUCCESS
		}
	}

	httpx.SendResponse(w, ld, response)
}
