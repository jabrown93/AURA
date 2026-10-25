package routes_download

import (
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type GetAllDownloadQueueItems_Response struct {
	Jobs []database.DownloadQueueJob `json:"jobs"`
}

// GetAllDownloadQueueItems godoc
// @Summary      Download Queue - Get Items
// @Description  Retrieve the current items in the download queue, each with its status (pending, processing, success, warning, error). This endpoint allows clients to monitor the progress of queued download tasks and identify any issues that may have occurred during processing.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetAllDownloadQueueItems_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/item [get]
func GetAllDownloadQueueItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Get Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetAllDownloadQueueItems_Response

	jobs, Err := database.GetAllDownloadQueueJobs(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Jobs = jobs
	httpx.SendResponse(w, ld, response)
}
