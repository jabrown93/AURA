package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type RemoveCollectionFromDownloadQueue_Request struct {
	Item models.CollectionQueueItem `json:"item"`
}

type RemoveCollectionFromDownloadQueue_Response struct {
	Result string `json:"result"`
}

// RemoveCollectionFromDownloadQueue godoc
// @Summary      Download Queue - Remove Collection Item
// @Description  Remove a specific Collection entry from the download queue. This can be used to cancel pending collection downloads or clean up errored entries.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RemoveCollectionFromDownloadQueue_Request  true  "Queue Remove Collection Item Request"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RemoveCollectionFromDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/collection [delete]
func RemoveCollectionFromDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Remove Collection Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RemoveCollectionFromDownloadQueue_Request
	var response RemoveCollectionFromDownloadQueue_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Remove Collection Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Delete Collection Item", logging.LevelDebug)
	if req.Item.CollectionItem.RatingKey == "" || req.Item.CollectionItem.LibraryTitle == "" {
		validateAction.SetError("Invalid Delete Collection Item structure",
			"Ensure that the request body contains a Collection Item with RatingKey and LibraryTitle",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	deleted, Err := downloadqueue.RemoveCollectionFromQueue(ctx, req.Item)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if deleted > 0 {
		logAction.AppendResult("total_deleted", deleted)
	} else {
		logAction.AppendResult("total_deleted", 0)
		logAction.AppendResult("message", "No matching items found in the collection download queue")
	}

	response.Result = fmt.Sprintf("Removed %d item(s) from the collection download queue", deleted)
	httpx.SendResponse(w, ld, response)
}
