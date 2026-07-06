package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type RetryItemInDownloadQueue_Request struct {
	Item models.DBSavedItem `json:"item"`
}

type RetryItemInDownloadQueue_Response struct {
	Result string `json:"result"`
}

// RetryItemInDownloadQueue godoc
// @Summary      Download Queue - Retry Item
// @Description  Retry a failed (errored) Media Item in the download queue. The errored entry is atomically re-queued as an in-progress entry so the download worker reprocesses it on its next run.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RetryItemInDownloadQueue_Request  true  "Queue Retry Item Request"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RetryItemInDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/item/retry [post]
func RetryItemInDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Retry Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RetryItemInDownloadQueue_Request
	var response RetryItemInDownloadQueue_Response

	// Parse and validate request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Retry Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Validate the JSON structure
	validateAction := logAction.AddSubAction("Validate Retry Item", logging.LevelDebug)
	if req.Item.MediaItem.Title == "" || req.Item.MediaItem.LibraryTitle == "" || req.Item.MediaItem.TMDB_ID == "" || req.Item.MediaItem.RatingKey == "" {
		validateAction.SetError("Invalid Retry Item structure",
			"Ensure that the request body contains a valid Retry Item with all required fields",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	retried, Err := downloadqueue.RetryFromQueue(ctx, req.Item)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if retried > 0 {
		logAction.AppendResult("total_retried", retried)
	} else {
		logAction.AppendResult("total_retried", 0)
		logAction.AppendResult("message", "No matching errored items found in the download queue")
	}

	response.Result = fmt.Sprintf("Re-queued %d errored item(s) for download", retried)
	httpx.SendResponse(w, ld, response)
}
