package routes_ms

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type GetMediaItemDetails_Response struct {
	ServerType     string                    `json:"server_type"`
	MediaItem      models.MediaItem          `json:"media_item"`
	PosterSets     models.PosterSetsResponse `json:"poster_sets"`
	UserFollowHide []models.MediuxUserInfo   `json:"user_follow_hide"`
}

// GetMediaItemDetails godoc
// @Summary      Get Media Item Details
// @Description  Retrieve detailed information about a specific media item from the media server, including its metadata, associated poster sets, and user follow/hide status. This endpoint accepts a rating key as a query parameter to identify the media item and returns comprehensive details that can be used to display the media item information and related sets in the client application.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        rating_key query string true "Rating Key of the Media Item"
// @Param        return_type query string false "Return Type (full or item, default is full)"
// @Param        library_title query string false "Title of the library the Media Item is in"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetMediaItemDetails_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/item [get]
func GetMediaItemDetails(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Media Item Details", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetMediaItemDetails_Response

	actionGetQueryParams := logAction.AddSubAction("Get Query Params", logging.LevelTrace)
	ratingKey := r.URL.Query().Get("rating_key")
	returnType := r.URL.Query().Get("return_type")
	libraryTitle := r.URL.Query().Get("library_title")
	if ratingKey == "" {
		actionGetQueryParams.SetError("Missing query parameter: rating_key", "Make sure to provide a valid rating_key", nil)
		httpx.SendResponse(w, ld, response)
		return
	}
	if returnType == "" || (returnType != "full" && returnType != "item") {
		returnType = "full" // Default to full details
	}

	// Get the Media Item from the cache
	// If the library title is provided, get the item from that library, as some media servers (e.g. Silo)
	// use the same rating key for an item in every library it is in
	var mediaItem *models.MediaItem
	var found bool
	if libraryTitle != "" {
		mediaItem, found = cache.LibraryStore.GetMediaItemFromSectionByRatingKey(libraryTitle, ratingKey)
	}
	if !found {
		mediaItem, found = cache.LibraryStore.GetMediaItemByRatingKey(ratingKey)
	}
	if !found {
		actionGetQueryParams.SetError("Media item not found in cache", "Make sure the rating_key is correct and the media server is connected", map[string]any{
			"rating_key": ratingKey,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get detailed info from the media server
	found, Err := mediaserver.GetMediaItemDetails(ctx, mediaItem)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if !found {
		logAction.SetError("Media item details not found", "The media item details could not be retrieved from the media server", map[string]any{
			"rating_key": ratingKey,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	response.ServerType = config.Current.MediaServer.Type
	response.MediaItem = *mediaItem

	// If the return type is item, return only the media item details
	if returnType == "item" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// From here on out, we need to get more data (poster sets, user follow/hide)
	// If any of them fail, we just send what we have and make status warning
	// Get the all sets for this TMDB ID
	switch mediaItem.Type {
	case "show":
		showSets, showItems, Err := mediux.GetShowItemSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition)
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}
		response.PosterSets.Sets = showSets
		response.PosterSets.IncludedItems = showItems
	case "movie":
		setItems := map[string]models.IncludedItem{}
		// For Movies, we get Movie Sets and Movie Collection Sets
		movieSets, Err := mediux.GetMovieItemSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, &setItems)
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}
		response.PosterSets.Sets = movieSets
		collectionSets, Err := mediux.GetMovieItemCollectionSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, &setItems)
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}
		response.PosterSets.Sets = append(response.PosterSets.Sets, collectionSets...)
		response.PosterSets.IncludedItems = setItems
	default:
		logAction.SetError("Invalid Media Item Type", "The media item type is not valid for fetching sets", map[string]any{
			"item_type": mediaItem.Type,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get MediUX user follow/hide info
	userFollowHide, Err := mediux.GetUserFollowingAndHiding(ctx)
	if Err.Message != "" {
		logAction.Status = logging.StatusWarn
	} else {
		response.UserFollowHide = userFollowHide
	}

	httpx.SendResponse(w, ld, response)
}
