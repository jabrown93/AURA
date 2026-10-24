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

type GetAllCollectionChildrenItems_Response struct {
	CollectionItem models.CollectionItem   `json:"collection_item"`
	Sets           []models.SetRef         `json:"sets"`
	UserFollowHide []models.MediuxUserInfo `json:"user_follow_hide"`
}

// GetAllCollectionChildrenItems godoc
// @Summary      Get All Movie Collection Children Items And Posters
// @Description  Retrieve all child items of a movie collection from the media server, along with their associated posters. This endpoint accepts a query parameter to identify the collection and returns the child items contained within that collection, as well as any relevant poster sets for those items. This allows clients to display the contents of a movie collection along with visual representations (posters) for each item.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        rating_key query string true "Rating Key of the Collection"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetAllCollectionChildrenItems_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/collections/item [get]
func GetAllCollectionChildrenItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get All Movie Collection Children Items And Posters", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetAllCollectionChildrenItems_Response
	actionGetQueryParams := logAction.AddSubAction("Get Query Params", logging.LevelTrace)
	ratingKey := r.URL.Query().Get("rating_key")
	if ratingKey == "" {
		actionGetQueryParams.SetError("Missing query parameter: rating_key", "Make sure to provide a valid rating_key", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get the Collection Item from the cache
	collectionItem, found := cache.CollectionsStore.GetCollectionByRatingKey(ratingKey)
	if !found {
		actionGetQueryParams.SetError("Collection item not found in cache", "Make sure the rating_key is correct and the media server is connected", map[string]any{
			"rating_key": ratingKey,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Get all children items for the collection
	Err := mediaserver.GetCollectionChildrenItems(ctx, collectionItem)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	response.CollectionItem = *collectionItem

	// Get MediUX user follow/hide info
	userFollowHide, Err := mediux.GetUserFollowingAndHiding(ctx)
	if Err.Message != "" {
		logAction.Status = logging.StatusWarn
	} else {
		response.UserFollowHide = userFollowHide
	}
	response.UserFollowHide = userFollowHide

	// Get Poster Sets for the collection items
	// For Plex Servers, we need to get an array of unique TMDB IDs from the collection items
	// For Emby/Jellyfin, we can use the existing TMDB IDs in the collection items
	// For Silo, TMDB collections have their TMDB ID, other collections use their movies like Plex
	switch config.Current.MediaServer.Type {
	case "Plex":
		response.Sets, Err = mediux.GetCollectionImagesByMovieTMDBIDs(ctx, collectionMovieTMDBIDs(collectionItem))
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}

	case "Silo":
		if collectionItem.TMDB_ID != "" {
			response.Sets, Err = mediux.GetCollectionImagesByTMDBID(ctx, collectionItem.TMDB_ID)
		} else {
			response.Sets, Err = mediux.GetCollectionImagesByMovieTMDBIDs(ctx, collectionMovieTMDBIDs(collectionItem))
		}
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}

	case "Emby", "Jellyfin":
		response.Sets, Err = mediux.GetCollectionImagesByTMDBID(ctx, collectionItem.TMDB_ID)
		if Err.Message != "" {
			logAction.Status = logging.StatusWarn
			break
		}

	default:
		logAction.SetError("Unsupported Media Server Type", "The media server type is not supported for fetching collection items", map[string]any{
			"server_type": config.Current.MediaServer.Type,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Send the response with the collection items
	httpx.SendResponse(w, ld, response)
}

// collectionMovieTMDBIDs returns the TMDB IDs of a collection's movies
func collectionMovieTMDBIDs(collection *models.CollectionItem) []string {
	ids := []string{}
	for _, child := range collection.MediaItems {
		if child.Type == "movie" && child.TMDB_ID != "" {
			ids = append(ids, child.TMDB_ID)
		}
	}
	return ids
}
