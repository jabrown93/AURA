package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type AddCollectionToDownloadQueue_Request struct {
	Item models.CollectionQueueItem `json:"item"`
}

type AddCollectionToDownloadQueue_Response struct {
	Result string `json:"result"`
}

// AddCollectionToDownloadQueue godoc
// @Summary      Download Queue - Add Collection Item
// @Description  Add a Collection and its selected images to the collection download queue. The entry is processed by the download worker and removed from the queue once completed.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      AddCollectionToDownloadQueue_Request  true  "Queue Add Collection Item Request"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=AddCollectionToDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/collection [post]
func AddCollectionToDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Add Collection Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req AddCollectionToDownloadQueue_Request
	var response AddCollectionToDownloadQueue_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Add Collection Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Collection Queue Item", logging.LevelDebug)
	if req.Item.CollectionItem.RatingKey == "" || req.Item.CollectionItem.Title == "" || req.Item.CollectionItem.LibraryTitle == "" {
		validateAction.SetError("Invalid Collection Queue Item structure",
			"Ensure that the request body contains a Collection Item with RatingKey, Title, and LibraryTitle",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	if len(req.Item.Images) == 0 {
		validateAction.SetError("Invalid Collection Queue Item structure - No Images",
			"Ensure that the Collection Item contains at least one image to download",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	for _, image := range req.Item.Images {
		if image.ID == "" || image.Type == "" {
			validateAction.SetError("Invalid Collection Queue Item structure - Image missing ID/Type",
				"Ensure that each image in the Collection Item contains a valid ID and Type",
				map[string]any{
					"image": image,
				})
			validateAction.Complete()
			httpx.SendResponse(w, ld, response)
			return
		}
	}
	validateAction.Complete()

	addErr := downloadqueue.AddCollectionToQueue(ctx, req.Item)
	if addErr.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Collection item added to download queue"
	httpx.SendResponse(w, ld, response)
}
