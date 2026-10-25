package routes_ms

import (
	"aura/cache"
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"net/http"
)

type RefreshMediaItemMetadata_Response struct {
	Refreshed bool `json:"refreshed"`
}

// RefreshMediaItemMetadata godoc
// @Summary      Refresh Media Item Metadata
// @Description  Refresh the metadata of a media item on the media server. This endpoint accepts a rating_key for the media item to be refreshed and an optional refresh_rating_key to specify which metadata entry to refresh. It triggers a metadata refresh on the media server for the specified media item, allowing clients to update the displayed information for that item after changes have been made on the media server or to fix any metadata issues.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        rating_key query string true "Rating Key of the media item to refresh"
// @Param        refresh_rating_key query string true "Rating Key to specify which metadata entry to refresh"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=RefreshMediaItemMetadata_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/refresh [post]
func RefreshMediaItemMetadata(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Refresh Media Item Metadata", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response RefreshMediaItemMetadata_Response
	response.Refreshed = false

	// Get the query parameters
	actionGetQueryParams := logAction.AddSubAction("Get Query Parameters", logging.LevelTrace)
	ratingKey := r.URL.Query().Get("rating_key")
	refreshRatingKey := r.URL.Query().Get("refresh_rating_key")
	if ratingKey == "" || refreshRatingKey == "" {
		actionGetQueryParams.SetError("Query parameter 'rating_key' and 'refresh_rating_key' are required", "Make sure to provide valid rating_key and refresh_rating_key", map[string]any{
			"rating_key":         ratingKey,
			"refresh_rating_key": refreshRatingKey,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get the Media Item from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemByRatingKey(ratingKey)
	if !found {
		actionGetQueryParams.SetError("Media item not found in cache",
			"Make sure the rating_key is correct and the media server is connected",
			map[string]any{
				"rating_key": ratingKey,
			})
		httpx.SendResponse(w, ld, response)
		return
	}

	Err := mediaserver.RefreshMediaItemMetadata(ctx, mediaItem, refreshRatingKey, true)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Refreshed = true
	httpx.SendResponse(w, ld, response)
}
