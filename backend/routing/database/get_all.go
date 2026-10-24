package routes_db

import (
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
	"strconv"
	"strings"
)

type getAllItemsResponse struct {
	Items       []models.DBSavedItem `json:"items"`
	TotalItems  int                  `json:"total_items"`
	UniqueUsers []string             `json:"unique_users"`
}

// GetAllItems godoc
// @Summary      Get All Database Items
// @Description  Retrieve all media items and their associated poster sets from the database, with optional filtering and pagination.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        item_tmdb_id       query     string  false  "Filter by TMDB ID (partial match)"
// @Param        item_library_title query     string  false  "Filter by Library Title (partial match)"
// @Param        item_year          query     int     false  "Filter by Year"
// @Param        item_title         query     string  false  "Filter by Title (partial match)"
// @Param        library_titles     query     string  false  "Filter by Library Titles (comma-separated for multiple)"
// @Param        image_types        query     string  false  "Filter by Image Types (comma-separated for multiple)"
// @Param        autodownload       query     string  false  "Filter by Autodownload status (on, off, all)"
// @Param        multiset_only      query     bool    false  "Filter to only items with multiple poster sets"
// @Param        usernames          query     string  false  "Filter by Usernames (comma-separated for multiple)"
// @Param        items_per_page     query     int     false  "Number of items per page for pagination (default: 20, -1 for no pagination)"
// @Param        page_number        query     int     false  "Page number for pagination (default: 1)"
// @Param        sort_option        query     string  false  "Sort option (e.g., date_downloaded, title, year)"
// @Param        sort_order         query     string  false  "Sort order (asc or desc)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200               {object}   httpx.JSONResponse{data=getAllItemsResponse}
// @Failure      500  			   {object}   httpx.JSONResponse "Internal Server Error"
// @Router       /api/db [get]
func GetAllItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get All Database Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	dbFilter := models.DBFilter{}
	var response getAllItemsResponse

	getQueryParamsAction := logAction.AddSubAction("Get Query Parameters", logging.LevelDebug)
	// Query Param - TMDB ID (Search Term)
	dbFilter.ItemTMDB_ID = r.URL.Query().Get("item_tmdb_id")

	// Query Param - Library (Search Term)
	dbFilter.ItemLibraryTitle = r.URL.Query().Get("item_library_title")

	// Query Param - Year (Search Term)
	itemYearStr := r.URL.Query().Get("item_year")
	itemYear := 0
	if itemYearStr != "" {
		if val, err := strconv.Atoi(itemYearStr); err == nil {
			itemYear = val
		}
	}
	dbFilter.ItemYear = itemYear

	// Query Param - Title (Search Term)
	dbFilter.ItemTitle = r.URL.Query().Get("item_title")

	// Query Param - Library Sections (Filter)
	librarySectionsStr := r.URL.Query().Get("library_titles")
	var filteredLibraryTitles []string
	if librarySectionsStr != "" {
		filteredLibraryTitles = strings.Split(librarySectionsStr, ",")
	}
	dbFilter.LibraryTitles = filteredLibraryTitles

	// Query Param - Filtered Types (Filter)
	imageTypesStr := r.URL.Query().Get("image_types")
	var filteredTypes []string
	if imageTypesStr != "" {
		filteredTypes = strings.Split(imageTypesStr, ",")
	}
	dbFilter.ImageTypes = filteredTypes

	// Query Param - AutoDownload Only (Filter)
	filterAutodownload := r.URL.Query().Get("autodownload")
	if filterAutodownload == "" {
		filterAutodownload = "all"
	}
	if filterAutodownload != "all" && filterAutodownload != "on" && filterAutodownload != "off" && filterAutodownload != "none" {
		getQueryParamsAction.AppendWarning("message", "Invalid filterAutodownload value, defaulting to 'all'")
		filterAutodownload = "all"
	}
	dbFilter.Autodownload = filterAutodownload

	// Query Param - Multi-Set Only (Filter)
	multisetOnly := false
	multisetOnlyStr := r.URL.Query().Get("multiset_only")
	if strings.ToLower(multisetOnlyStr) == "true" {
		multisetOnly = true
	}
	dbFilter.MultiSetOnly = multisetOnly

	// Query Param - Filtered Usernames (Filter)
	filteredUsernamesStr := r.URL.Query().Get("usernames")
	var filteredUsernames []string
	if filteredUsernamesStr != "" {
		filteredUsernames = strings.Split(filteredUsernamesStr, ",")
	} else if fuStr := r.URL.Query().Get("filtered_usernames"); fuStr != "" {
		filteredUsernames = strings.Split(fuStr, ",")
	}
	dbFilter.Usernames = filteredUsernames

	// Query Param - Media Item On Server (Filter)
	dbFilter.MediaItemOnServer = r.URL.Query().Get("media_item_on_server")
	if dbFilter.MediaItemOnServer != "" && dbFilter.MediaItemOnServer != "true" && dbFilter.MediaItemOnServer != "false" {
		getQueryParamsAction.AppendWarning("message", "Invalid media_item_on_server value, defaulting to empty (no filter)")
		dbFilter.MediaItemOnServer = ""
	}

	// Query Param - Pagination & Sorting
	// Items Per Page (Default: 20, -1 for no pagination)
	// Page Number (Default: 1)
	// Sort Option (Default: date_downloaded)
	// Sort Order (Default: desc)
	itemsPerPage := 20
	pageNumber := 1
	sortOption := "date_downloaded"
	sortOrder := "desc"
	ippStr := r.URL.Query().Get("items_per_page")
	if ippStr != "" {
		if val, err := strconv.Atoi(ippStr); err == nil {
			itemsPerPage = val
		}
	}
	pnStr := r.URL.Query().Get("page_number")
	if pnStr != "" {
		if val, err := strconv.Atoi(pnStr); err == nil {
			pageNumber = val
		}
	}
	sortOption = r.URL.Query().Get("sort_option")
	sortOrder = r.URL.Query().Get("sort_order")
	getQueryParamsAction.Complete()
	dbFilter.ItemsPerPage = itemsPerPage
	dbFilter.PageNumber = pageNumber
	dbFilter.SortOption = sortOption
	dbFilter.SortOrder = sortOrder

	// Get the items from the database
	out, Err := database.GetAllSavedSets(ctx, dbFilter)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	totalItems := out.Total

	// // Get Saved Sets Count
	// totalSets, ErrCount := database.GetCountSavedSets(ctx)
	// if ErrCount.Message != "" {
	// 	httpx.SendResponse(w, ld, response)
	// 	return
	// }

	// Get a list of unique users
	uniqueUsers, Err := database.GetAllUniqueUsers(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Items = out.Items
	response.TotalItems = totalItems
	response.UniqueUsers = uniqueUsers
	httpx.SendResponse(w, ld, response)
}
