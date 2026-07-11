package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type GetAllCollectionDownloadQueueItems_Response struct {
	InProgressEntries []models.CollectionQueueItem `json:"in_progress_entries"`
	WarningEntries    []models.CollectionQueueItem `json:"warning_entries"`
	ErrorEntries      []models.CollectionQueueItem `json:"error_entries"`
}

// GetAllCollectionDownloadQueueItems godoc
// @Summary      Download Queue - Get Collection Items
// @Description  Retrieve the current items in the collection download queue, categorized by their status (in-progress, warning, error).
// @Tags         Download
// @Accept       json
// @Produce      json
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetAllCollectionDownloadQueueItems_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/collection [get]
func GetAllCollectionDownloadQueueItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Get Collection Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetAllCollectionDownloadQueueItems_Response

	inProgressItems, warningItems, errorItems, Err := downloadqueue.GetCollectionQueueItems(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.InProgressEntries = inProgressItems
	response.WarningEntries = warningItems
	response.ErrorEntries = errorItems
	httpx.SendResponse(w, ld, response)
}
