package routes_images

import (
	"aura/config"
	"aura/logging"
	"aura/utils"
	"aura/utils/httpx"
	"fmt"
	"net/http"
	"path"
)

type DeleteTempImages_Response struct {
	Message string `json:"message"`
}

// DeleteTempImages godoc
// @Summary      Delete Temporary Images
// @Description  Clear all temporary images from the server's temp-images directory. This endpoint is useful for maintenance and cleanup of temporary files that are no longer needed.
// @Tags         Images
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=DeleteTempImages_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/temp [delete]
func DeleteTempImages(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Delete Temp Images", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response DeleteTempImages_Response

	clearCount, Err := utils.ClearAllFilesFromFolder(ctx, path.Join(config.ConfigPath, "temp-images"))
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if clearCount == 0 {
		response.Message = "No temporary images to delete"
		httpx.SendResponse(w, ld, response)
		return
	} else {
		logAction.AppendResult("cleared_files", clearCount)
	}

	response.Message = fmt.Sprintf("Cleared %d temporary images", clearCount)
	httpx.SendResponse(w, ld, response)
}
