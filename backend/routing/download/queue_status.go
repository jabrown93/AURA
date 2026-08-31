package routes_download

import (
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
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetDownloadQueueStatus_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue [get]
func GetDownloadQueueStatus(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Get Status", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	latestInfo := downloadqueue.GetLatestInfo()

	var response GetDownloadQueueStatus_Response
	response.Time = latestInfo.Time
	response.Status = latestInfo.Status
	response.Message = latestInfo.Message
	response.Warnings = latestInfo.Warnings
	response.Errors = latestInfo.Errors

	httpx.SendResponse(w, ld, response)
}
