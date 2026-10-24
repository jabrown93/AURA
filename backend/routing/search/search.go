package routes_search

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
	"strconv"
	"strings"
)

type HandleSearch_Response struct {
	SearchQuery                   string                  `json:"search_query"`
	MediaItems                    []models.MediaItem      `json:"media_items"`
	MediaItemsLastFullUpdate      int64                   `json:"media_items_last_full_update"`
	CollectionItems               []models.CollectionItem `json:"collection_items"`
	CollectionItemsLastFullUpdate int64                   `json:"collection_items_last_full_update"`
	MediuxUsernames               []models.MediuxUserInfo `json:"mediux_usernames"`
	MediuxUsernamesLastFullUpdate int64                   `json:"mediux_usernames_last_full_update"`
	SavedSets                     []models.DBSavedItem    `json:"saved_sets"`
}

// HandleSearch godoc
// @Summary      Search
// @Description  Perform a search across media items, collection items, Mediux users, and saved sets based on the provided query and filters. The search supports filtering by year, library section, and unique identifiers (TMDB ID or RatingKey) using specific query syntax. The response includes matching media items, collection items, Mediux users, and saved sets, along with metadata about the last full update for each category to help clients determine if they need to refresh their cached data.
// @Tags         Search
// @Accept       json
// @Produce      json
// @Param        query query string true "The search query string with optional filters (e.g., 'Inception y:2010 l:Movies id:12345')"
// @Param        search_media_items query bool false "Whether to include media items in the search results"
// @Param        search_collection_items query bool false "Whether to include collection items in the search results"
// @Param        search_mediux_users query bool false "Whether to include Mediux users in the search results"
// @Param        search_saved_sets query bool false "Whether to include saved sets in the search results"
// @Success	  200  {object}  httpx.JSONResponse{data=HandleSearch_Response}
// @Failure	  500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/search [get]
func HandleSearch(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Search Handler", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response HandleSearch_Response

	// Get the Search Query Parameter
	searchQuery := r.URL.Query().Get("query")
	if searchQuery == "" {
		return
	}

	// Get Filters from Query
	searchMediaItems := r.URL.Query().Get("search_media_items") == "true"
	searchCollectionItems := r.URL.Query().Get("search_collection_items") == "true"
	searchMediuxUsers := r.URL.Query().Get("search_mediux_users") == "true"
	searchSavedSets := r.URL.Query().Get("search_saved_sets") == "true"

	if !searchMediaItems && !searchCollectionItems && !searchMediuxUsers && !searchSavedSets {
		return
	}

	yearFilter, hasYearFilter := extractYearFromQuery(searchQuery)
	libraryFilter, hasLibraryFilter := extractLibraryFromQuery(searchQuery)
	idFilter, hasIDFilter := extractIDFromQuery(searchQuery)
	cleanedQuery := removeOtherFiltersFromQuery(searchQuery)

	var yearFilterInt int
	if hasYearFilter {
		var err error
		yearFilterInt, err = strconv.Atoi(yearFilter)
		if err != nil {
			logging.LOGGER.Warn().Timestamp().Msgf("Invalid year filter: %s", yearFilter)
			hasYearFilter = false
		}
	}

	logging.Dev().Timestamp().
		Str("query_original", searchQuery).
		Str("query_cleaned", cleanedQuery).
		Str("library_filter", libraryFilter).
		Str("year_filter", yearFilter).
		Str("id_filter", idFilter).
		Bool("search_media_items", searchMediaItems).
		Bool("search_collection_items", searchCollectionItems).
		Bool("search_mediux_users", searchMediuxUsers).
		Bool("search_saved_sets", searchSavedSets).
		Msg("Processing search query")

	var Err logging.LogErrorInfo

	response.MediaItemsLastFullUpdate = cache.LibraryStore.LastFullUpdate
	response.MediuxUsernamesLastFullUpdate = cache.MediuxUsers.LastFullUpdate
	response.CollectionItemsLastFullUpdate = cache.CollectionsStore.LastFullUpdate

	if searchMediaItems {
		maxPerSection := 25
		allSections := cache.LibraryStore.GetAllSectionsSortedByTitle()
		for _, section := range allSections {
			count := 0
			for _, item := range section.MediaItems {
				if hasLibraryFilter && !strings.EqualFold(item.LibraryTitle, libraryFilter) {
					continue
				}
				if hasYearFilter && item.Year != yearFilterInt {
					continue
				}
				if hasIDFilter && (item.TMDB_ID != idFilter && item.RatingKey != idFilter) {
					continue
				}
				queryWords := strings.Fields(strings.ToLower(cleanedQuery))
				titleLower := strings.ToLower(item.Title)
				allWordsMatch := true
				for _, word := range queryWords {
					if !strings.Contains(titleLower, word) {
						allWordsMatch = false
						break
					}
				}
				if allWordsMatch {
					response.MediaItems = append(response.MediaItems, item)
					count++
					if count >= maxPerSection {
						break
					}
				}
			}
		}
	}

	if searchCollectionItems {
		maxTotal := 50
		allCollections := cache.CollectionsStore.GetAllCollections()
		for _, collection := range allCollections {
			count := 0
			if hasLibraryFilter && !strings.EqualFold(collection.LibraryTitle, libraryFilter) {
				continue
			}
			queryWords := strings.Fields(strings.ToLower(cleanedQuery))
			titleLower := strings.ToLower(collection.Title)
			allWordsMatch := true
			for _, word := range queryWords {
				if !strings.Contains(titleLower, word) {
					allWordsMatch = false
					break
				}
			}
			if allWordsMatch {
				response.CollectionItems = append(response.CollectionItems, collection)
				count++
				if len(response.CollectionItems) >= maxTotal {
					break
				}
			}
			if len(response.CollectionItems) >= maxTotal {
				break
			}
		}
	}

	if searchMediuxUsers {
		// Get MediUX Usernames and filter based on query
		if cleanedQuery != "" {
			response.MediuxUsernames, Err = mediux.SearchUsersByUsername(ctx, cleanedQuery)
			if Err.Message != "" {
				ld.Status = logging.StatusWarn
				httpx.SendResponse(w, ld, response)
				return
			}
		}
	}

	if searchSavedSets {
		// Get Saved Sets and filter based on query
		out, Err := database.GetAllSavedSets(ctx, models.DBFilter{
			ItemTMDB_ID:      idFilter,
			ItemTitle:        cleanedQuery,
			ItemLibraryTitle: libraryFilter,
			ItemYear:         yearFilterInt,
			ItemsPerPage:     25,
		})
		if Err.Message != "" {
			ld.Status = logging.StatusWarn
			httpx.SendResponse(w, ld, response)
			return
		}
		response.SavedSets = out.Items
	}

	logging.LOGGER.Trace().Timestamp().
		Int("found_media_items", len(response.MediaItems)).
		Int("found_mediux_usernames", len(response.MediuxUsernames)).
		Int("found_saved_sets", len(response.SavedSets)).
		Msg("Search completed")

	httpx.SendResponse(w, ld, response)
}

func extractYearFromQuery(query string) (string, bool) {
	prefix := "y:"
	suffix := ":"
	startIdx := strings.Index(strings.ToLower(query), prefix)
	if startIdx == -1 {
		return "", false
	}
	startIdx += len(prefix)
	endIdx := strings.Index(query[startIdx:], suffix)
	if endIdx == -1 {
		return "", false
	}
	year := query[startIdx : startIdx+endIdx]
	return year, true
}

func extractLibraryFromQuery(query string) (string, bool) {
	prefix := "l:"
	suffix := ":"
	startIdx := strings.Index(strings.ToLower(query), prefix)
	if startIdx == -1 {
		return "", false
	}
	startIdx += len(prefix)
	endIdx := strings.Index(query[startIdx:], suffix)
	if endIdx == -1 {
		return "", false
	}
	library := query[startIdx : startIdx+endIdx]
	return library, true
}

func extractIDFromQuery(query string) (string, bool) {
	prefix := "id:"
	suffix := ":"
	startIdx := strings.Index(strings.ToLower(query), prefix)
	if startIdx == -1 {
		return "", false
	}
	startIdx += len(prefix)
	endIdx := strings.Index(query[startIdx:], suffix)
	if endIdx == -1 {
		return "", false
	}
	id := query[startIdx : startIdx+endIdx]
	return id, true
}

func removeOtherFiltersFromQuery(query string) string {
	filters := []string{"y:", "l:", "id:"}
	cleanedQuery := query

	for _, filter := range filters {
		for {
			startIdx := strings.Index(strings.ToLower(cleanedQuery), filter)
			if startIdx == -1 {
				break
			}
			endIdx := strings.Index(cleanedQuery[startIdx+len(filter):], ":")
			if endIdx == -1 {
				break
			}
			cleanedQuery = strings.TrimSpace(cleanedQuery[:startIdx] + cleanedQuery[startIdx+len(filter)+endIdx+1:])
		}
	}

	return strings.TrimSpace(cleanedQuery)
}
