package routes_mediux

import (
	"aura/cache"
	"aura/creator"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
	"strconv"
)

// GetCreatorSets godoc
// @Summary      Get Creator Sets for a Library
// @Description  A creator's sets matched against one library, filtered, searched and sorted like the user page, with one page of one tab and the counts of every tab. The creator's sets are kept for 10 minutes.
// @Tags         MediUX
// @Produce      json
// @Param        username  query  string  true   "MediUX username"
// @Param        library   query  string  false  "Library to match against. Without it, only the libraries are listed"
// @Param        tab       query  string  false  "showSets, movieSets, collectionSets or boxSets"
// @Param        search    query  string  false  "Set title or the title of an item in the set"
// @Param        in_db     query  string  false  "inDB, notInDB or otherSetInDB"
// @Param        sort      query  string  false  "title or dateLastUpdate"
// @Param        order     query  string  false  "asc or desc"
// @Param        page      query  int     false  "Page, from 1"
// @Param        per_page  query  int     false  "Sets per page"
// @Param        refresh   query  bool    false  "Get the creator's sets from MediUX again"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=creator.Result}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediux/sets/user [get]
func GetCreatorSets(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Creator Sets", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)

	query := r.URL.Query()
	username := query.Get("username")
	if username == "" {
		logAction.SetError("A username is needed", "Add the MediUX username to the request", nil)
		httpx.SendResponse(w, ld, creator.Result{})
		return
	}
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))
	refresh, _ := strconv.ParseBool(query.Get("refresh"))

	result, Err := creator.Find(ctx, cache.LibraryStore, creator.Query{
		Username: username,
		Library:  query.Get("library"),
		Tab:      query.Get("tab"),
		Search:   query.Get("search"),
		InDB:     query.Get("in_db"),
		Sort:     query.Get("sort"),
		Order:    query.Get("order"),
		Page:     page,
		PerPage:  perPage,
		Refresh:  refresh,
	})
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
	}
	httpx.SendResponse(w, ld, result)
}
