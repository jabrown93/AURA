package routes_ms

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"net/http"
	"strconv"
)

type RateMediaItem_Response struct {
	Rating float64 `json:"rating"`
	Rated  bool    `json:"rated"`
}

// RateMediaItem godoc
// @Summary      Rate Media Item
// @Description  Rate a media item on the media server. This endpoint allows clients to submit a user rating for a specific media item, which will be sent to the media server (currently only supported for Plex and Silo). The rating should be a number between 0 and 5, and it will be converted to the appropriate scale for the media server before being submitted.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        rating_key   query     string  true  "The rating key of the media item to rate"
// @Param        rating       query     string  true  "The user rating for the media item (0-5)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=RateMediaItem_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/rate [patch]
func RateMediaItem(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Rate Media Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response RateMediaItem_Response

	if config.Current.MediaServer.Type != "Plex" && config.Current.MediaServer.Type != "Silo" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get the query parameters
	actionGetQueryParams := logAction.AddSubAction("Get Query Parameters", logging.LevelTrace)
	ratingKey := r.URL.Query().Get("rating_key")
	userRatingStr := r.URL.Query().Get("rating")
	if ratingKey == "" || userRatingStr == "" {
		actionGetQueryParams.SetError("Missing query parameters: rating_key or rating", "Make sure to provide a valid rating_key and rating", map[string]any{
			"rating_key": ratingKey,
			"rating":     userRatingStr,
		})
		httpx.SendResponse(w, ld, response)
		return
	}
	userRating, err := strconv.ParseFloat(userRatingStr, 64)
	if err != nil {
		actionGetQueryParams.SetError("Invalid rating value", "Rating must be a number between 0 and 5", map[string]any{
			"rating": userRatingStr,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	if userRating < 0 || userRating > 5 {
		actionGetQueryParams.SetError("Invalid rating value", "Rating must be between 0 and 5", map[string]any{
			"rating": userRating,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get the Media Item from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemByRatingKey(ratingKey)
	if !found {
		actionGetQueryParams.SetError("Media item not found in cache", "Make sure the rating_key is correct and the media server is connected", map[string]any{
			"rating_key": ratingKey,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Convert rating to scale of 10 for the media server
	userRating = userRating * 2

	Err := mediaserver.RateMediaItem(ctx, mediaItem, userRating)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Rated = true
	response.Rating = userRating
	httpx.SendResponse(w, ld, response)
}
