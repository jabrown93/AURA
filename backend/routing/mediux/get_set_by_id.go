package routes_mediux

import (
	"aura/kometa"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type GetSetByID_Response struct {
	Set           models.SetRef                  `json:"set"`
	IncludedItems map[string]models.IncludedItem `json:"included_items"`
}

// GetSetByID godoc
// @Summary      Get Mediux Set By ID
// @Description  Retrieve a specific item set (such as a show set for TV shows or a movie set/collection for movies) by its unique identifier. This endpoint accepts query parameters to identify the set, including the set ID, set type (show, movie, or collection), and the library section it belongs to. The response includes details about the set and any related media items that are part of the set, allowing clients to display comprehensive information about the set and its contents in the UI.
// @Tags         Mediux
// @Accept       json
// @Produce      json
// @Param        set_id query string true "Unique identifier of the item set"
// @Param        set_type query string true "Type of the item set (show, movie, or collection)"
// @Param        tmdb_id query string false "TMDB ID of the media item (required for collection sets)"
// @Param        item_library_title query string true "Title of the library the set belongs to"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true"
// @Success	  200  {object}  httpx.JSONResponse{data=GetSetByID_Response}
// @Failure	  500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediux/set [get]
func GetSetByID(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get MediUX Set By ID", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetSetByID_Response

	actionGetQueryParams := logAction.AddSubAction("Get all query params", logging.LevelTrace)
	// Get the following information from the URL
	// Set ID
	// Library Section
	// Item Type
	// TMDB ID
	setID := r.URL.Query().Get("set_id")
	setType := r.URL.Query().Get("set_type")
	tmdbID := r.URL.Query().Get("tmdb_id")
	itemLibraryTitle := r.URL.Query().Get("item_library_title")
	// Validate the set ID, library section, item type, and TMDB ID
	if setID == "" || setType == "" || itemLibraryTitle == "" || (setType != "show" && setType != "movie" && setType != "collection") {
		actionGetQueryParams.SetError("Missing Query Parameters", "One or more required query parameters are missing or invalid",
			map[string]any{
				"set_id":           setID,
				"set_type":         setType,
				"valid_item_types": []string{"show", "movie", "collection"},
			})
		httpx.SendResponse(w, ld, response)
		return
	}
	// Kometa-imported sets are stored on disk, not on MediUX; there is nothing to fetch.
	if kometa.IsKometaSetID(setID) {
		actionGetQueryParams.SetError("Kometa-imported set", "Kometa-imported sets are local assets and have no MediUX set to retrieve",
			map[string]any{"set_id": setID})
		httpx.SendResponse(w, ld, response)
		return
	}
	actionGetQueryParams.Complete()

	switch setType {
	case "show":
		showSet, includedItems, Err := mediux.GetShowSetByID(ctx, setID, itemLibraryTitle)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Set = showSet
		response.IncludedItems = includedItems
	case "movie":
		movieSet, includedItems, Err := mediux.GetMovieSetByID(ctx, setID, itemLibraryTitle)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Set = movieSet
		response.IncludedItems = includedItems

	case "collection":
		collectionSet, includedItems, Err := mediux.GetMovieCollectionSetByID(ctx, setID, tmdbID, itemLibraryTitle, true)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.Set = collectionSet
		response.IncludedItems = includedItems
	}

	httpx.SendResponse(w, ld, response)
}
