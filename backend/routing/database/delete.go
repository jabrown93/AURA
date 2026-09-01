package routes_db

import (
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type DeleteItemFromDB_Response struct {
	Message string `json:"message"`
}

// DeleteItemFromDB godoc
// @Summary      Delete Item From Database
// @Description  Delete a Media Item and all associated Poster Sets from the database based on TMDB ID and Library Title.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        tmdb_id       query     string  true  "TMDB ID of the Media Item"
// @Param        library_title  query     string  true  "Library Title of the Media Item"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200            {object}  httpx.JSONResponse{data=DeleteItemFromDB_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db [delete]
func DeleteItemFromDB(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Delete Item From Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response DeleteItemFromDB_Response

	// Get the query parameters
	tmdbID := r.URL.Query().Get("tmdb_id")
	libraryTitle := r.URL.Query().Get("library_title")

	// Validate the parameters
	if tmdbID == "" || libraryTitle == "" {
		ld.AddAction("Invalid parameters for deleting item from database", logging.LevelError)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Delete the item
	Err := database.DeleteAllPosterSetsForMediaItem(ctx, tmdbID, libraryTitle)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Message = "Deleted saved item and associated poster sets successfully"
	httpx.SendResponse(w, ld, response)
}
