package routes_mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type getItemSetsResponse struct {
	Sets          []models.SetRef                `json:"sets"`
	IncludedItems map[string]models.IncludedItem `json:"included_items"`
}

// GetItemSets godoc
// @Summary      Get Mediux Item Sets
// @Description  Retrieve item sets for a specific media item in the library. This endpoint accepts query parameters to identify the media item and its library, and returns any related item sets (such as show sets for TV shows or movie sets/collections for movies) that the media item belongs to, allowing clients to display related items and collections in the UI.
// @Tags         Mediux
// @Accept       json
// @Produce      json
// @Param        tmdb_id query string true "TMDB ID of the media item"
// @Param        item_type query string true "Type of the media item (movie or show)"
// @Param        item_library_title query string true "Title of the library the media item belongs to"
// @Param        edition query string false "Edition of the media item (e.g. Director's Cut), empty for the standard edition"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success	  200  {object}  httpx.JSONResponse{data=getItemSetsResponse}
// @Failure	  500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediux/sets/item [get]
func GetItemSets(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Mediux Item Sets", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response getItemSetsResponse
	actionGetQueryParams := ld.AddAction("Get all query params", logging.LevelTrace)
	tmdbID := r.URL.Query().Get("tmdb_id")
	itemType := r.URL.Query().Get("item_type")
	itemLibraryTitle := r.URL.Query().Get("item_library_title")
	edition := r.URL.Query().Get("edition")
	if tmdbID == "" || itemType == "" || itemLibraryTitle == "" {
		actionGetQueryParams.SetError("Missing Query Parameters", "One or more required query parameters are missing",
			map[string]any{
				"tmdb_id":            tmdbID,
				"item_type":          itemType,
				"item_library_title": itemLibraryTitle,
			})
		httpx.SendResponse(w, ld, response)
		return
	}
	actionGetQueryParams.Complete()

	// If cache is empty, return false
	actionCheckCache := ld.AddAction("Check library cache", logging.LevelTrace)
	if cache.LibraryStore.IsEmpty() {
		actionCheckCache.SetError("Library Cache Empty", "The library cache is empty, cannot fetch sets", nil)
		httpx.SendResponse(w, ld, response)
		return
	}
	actionCheckCache.Complete()

	switch itemType {
	case "show":
		// For Shows, we get just Show Sets
		showSets, showItems, Err := mediux.GetShowItemSets(ctx, tmdbID, itemLibraryTitle, edition)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Sets = showSets
		response.IncludedItems = showItems
	case "movie":
		setItems := map[string]models.IncludedItem{}
		// For Movies, we get Movie Sets and Movie Collection Sets
		movieSets, Err := mediux.GetMovieItemSets(ctx, tmdbID, itemLibraryTitle, edition, &setItems)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Sets = movieSets
		collectionSets, Err := mediux.GetMovieItemCollectionSets(ctx, tmdbID, itemLibraryTitle, edition, &setItems)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Sets = append(response.Sets, collectionSets...)
		if len(movieSets) == 0 && len(collectionSets) == 0 && len(setItems) == 0 {
			logAction.SetError("No Sets Found", "No movie sets or collection sets found for the provided TMDB ID", map[string]any{
				"tmdb_id": tmdbID,
			})
			httpx.SendResponse(w, ld, response)
			return
		}
		response.IncludedItems = setItems
	default:
		logAction.SetError("Invalid Item Type", "The provided item type is not valid", map[string]any{
			"item_type": itemType,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	httpx.SendResponse(w, ld, response)
}
