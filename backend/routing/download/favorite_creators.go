package routes_download

import (
	"aura/cache"
	autodownload "aura/download/auto"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

// DownloadFromFavoriteCreators godoc
// @Summary      Download Sets from Favorite Creators for All Items
// @Description  For every item of the main media server without a saved set, adds the best set from the favorite creators to the Download Queue. Only runs when Automatically Download Images for New Items is on.
// @Tags         Download
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=autodownload.FavoriteCreatorsResult}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/favorite-creators [post]
func DownloadFromFavoriteCreators(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Sets from Favorite Creators for All Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	result, Err := autodownload.DownloadFromFavoriteCreators(ctx, cache.LibraryStore.GetAllMediaItems(), models.SourceFavoriteCreatorsAllItems)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
	}
	httpx.SendResponse(w, ld, result)
}
