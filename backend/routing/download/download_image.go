package routes_download

import (
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type DownloadImageFileForMediaItem_Request struct {
	ImageFile models.ImageFile `json:"image_file"`
	MediaItem models.MediaItem `json:"media_item"`
}

type DownloadImageFileForMediaItem_Response struct {
	Result string `json:"result"`
}

// DownloadImageFileForMediaItem godoc
// @Summary      Download Image
// @Description  Download and apply an image for a Media Item in the media server.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      DownloadImageFileForMediaItem_Request  true  "Download Image Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=DownloadImageFileForMediaItem_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/image/item [post]
func DownloadImageFileForMediaItem(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Image", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req DownloadImageFileForMediaItem_Request
	var response DownloadImageFileForMediaItem_Response

	// Parse the request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Download Image - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	actionValidate := logAction.AddSubAction("Validate Request Body Fields", logging.LevelDebug)
	// Make sure that the mediaItem has the following fields set
	// 1. MediaItem.RatingKey
	// 2. MediaItem.LibraryTitle
	if req.MediaItem.RatingKey == "" || req.MediaItem.LibraryTitle == "" {
		actionValidate.SetError("Invalid Media Item Data", "RatingKey and LibraryTitle are required in Media Item", map[string]any{
			"rating_key":    req.MediaItem.RatingKey,
			"library_title": req.MediaItem.LibraryTitle,
		})
		actionValidate.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure that the ImageFile has the following fields set
	// 1. ImageFile.ID
	// 2. ImageFile.Type
	if req.ImageFile.ID == "" || req.ImageFile.Type == "" {
		actionValidate.SetError("Invalid Image File Data", "ID and Type are required in Image File", map[string]any{
			"id":   req.ImageFile.ID,
			"type": req.ImageFile.Type,
		})
		actionValidate.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	actionValidate.Complete()

	// Make the download and apply the image
	Err = mediaserver.DownloadApplyImageToMediaItem(ctx, &req.MediaItem, req.ImageFile)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = fmt.Sprintf("Sucessfully downloaded %s", utils.GetFileDownloadName(req.MediaItem.Title, req.ImageFile))
	httpx.SendResponse(w, ld, response)
}
