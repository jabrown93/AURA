package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type RetryCollectionInDownloadQueue_Request struct {
	Item models.CollectionQueueItem `json:"item"`
}

type RetryCollectionInDownloadQueue_Response struct {
	Result string `json:"result"`
}

// RetryCollectionInDownloadQueue godoc
// @Summary      Download Queue - Retry Collection Item
// @Description  Retry a failed (errored) Collection entry in the download queue. The errored entry is atomically re-queued as an in-progress entry so the download worker reprocesses it on its next run.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RetryCollectionInDownloadQueue_Request  true  "Queue Retry Collection Item Request"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RetryCollectionInDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/collection/retry [post]
func RetryCollectionInDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Retry Collection Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RetryCollectionInDownloadQueue_Request
	var response RetryCollectionInDownloadQueue_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Retry Collection Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Retry Collection Item", logging.LevelDebug)
	if req.Item.CollectionItem.RatingKey == "" || req.Item.CollectionItem.LibraryTitle == "" {
		validateAction.SetError("Invalid Retry Collection Item structure",
			"Ensure that the request body contains a Collection Item with RatingKey and LibraryTitle",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	retried, Err := downloadqueue.RetryCollectionFromQueue(ctx, req.Item)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if retried > 0 {
		logAction.AppendResult("total_retried", retried)
	} else {
		logAction.AppendResult("total_retried", 0)
		logAction.AppendResult("message", "No matching errored items found in the collection download queue")
	}

	response.Result = fmt.Sprintf("Re-queued %d errored collection item(s) for download", retried)
	httpx.SendResponse(w, ld, response)
}
