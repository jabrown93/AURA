package routes_db

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type ignoreItemResponse struct {
	Ignored      bool   `json:"ignored"`
	TmdbID       string `json:"tmdb_id"`
	LibraryTitle string `json:"library_title"`
	Mode         string `json:"mode,omitempty"`         // e.g., "always", "until-set-available", "until-new-set-available"
	CurrentSets  string `json:"current_sets,omitempty"` // comma-separated list of current sets for the item, used for temporary ignore modes
}

// IgnoreItem godoc
// @Summary      Ignore Item In Database
// @Description  Mark a Media Item as ignored in the database, preventing it from being processed by other parts of the application. The ignore can be temporary or permanent based on the mode parameter.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        tmdb_id       query     string  true  "TMDB ID of the Media Item"
// @Param        library_title  query     string  true  "Library Title of the Media Item"
// @Param        mode           query     string  true  "Ignore mode (e.g., 'always' for permanent ignore, 'until-set-available' for temporary ignore until a set is available, 'until-new-set-available' for temporary ignore until a new set is available)"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200            {object}  httpx.JSONResponse{data=ignoreItemResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db/ignore [patch]
func IgnoreItemInDB(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Ignore Item In Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response ignoreItemResponse

	// Get query parameters
	tmdbID := r.URL.Query().Get("tmdb_id")
	libraryTitle := r.URL.Query().Get("library_title")
	mode := r.URL.Query().Get("mode")                // e.g., "always", "until-set-available", "until-new-set-available"
	currentSets := r.URL.Query().Get("current_sets") // comma-separated list of current sets for the item, used for temporary ignore modes

	if tmdbID == "" || libraryTitle == "" || mode == "" {
		logAction.SetError("Missing required query parameters", "TMDB ID, Library Title, and Mode are required",
			map[string]any{
				"tmdb_id":       tmdbID,
				"library_title": libraryTitle,
				"mode":          mode,
				"current_sets":  currentSets,
			})
		httpx.SendResponse(w, ld, response)
		return
	} else if mode != "always" && mode != "until-set-available" && mode != "until-new-set-available" {
		logAction.SetError("Invalid mode parameter", "Ignore mode must be 'always', 'until-set-available', or 'until-new-set-available'", map[string]any{
			"mode": mode,
		})
		httpx.SendResponse(w, ld, response)
		return
	} else if mode == "until-new-set-available" && currentSets == "" {
		logAction.SetError("Missing current_sets parameter for temporary ignore mode", "current_sets is required when mode is 'until-new-set-available'", map[string]any{
			"mode":         mode,
			"current_sets": currentSets,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	Err := database.IgnoreMediaItem(ctx, tmdbID, libraryTitle, mode, currentSets)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	cache.LibraryStore.SetIgnored(libraryTitle, tmdbID, true, mode)

	response.Ignored = true
	response.TmdbID = tmdbID
	response.LibraryTitle = libraryTitle
	response.Mode = mode
	response.CurrentSets = currentSets
	httpx.SendResponse(w, ld, response)
}

// StopIgnoringItem godoc
// @Summary      Stop Ignoring Item In Database
// @Description  Remove the ignored status from a Media Item in the database, allowing it to be processed by other parts of the application again.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        tmdb_id       query     string  true  "TMDB ID of the Media Item"
// @Param        library_title  query     string  true  "Library Title of the Media Item"
// @Success      200            {object}  httpx.JSONResponse{data=ignoreItemResponse}
// @Failure      500            {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db/ignore/stop [patch]
func StopIgnoringItemInDB(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Stop Ignoring Item In Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response ignoreItemResponse

	// Get query parameters
	tmdbID := r.URL.Query().Get("tmdb_id")
	libraryTitle := r.URL.Query().Get("library_title")

	if tmdbID == "" || libraryTitle == "" {
		logAction.SetError("Missing required query parameters", "TMDB ID and Library Title are required",
			map[string]any{
				"tmdb_id":       tmdbID,
				"library_title": libraryTitle,
			})
		httpx.SendResponse(w, ld, response)
		return
	}

	Err := database.StopIgnoringMediaItem(ctx, tmdbID, libraryTitle)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	cache.LibraryStore.SetIgnored(libraryTitle, tmdbID, false, "")

	response.Ignored = false
	response.TmdbID = tmdbID
	response.LibraryTitle = libraryTitle
	httpx.SendResponse(w, ld, response)
}
