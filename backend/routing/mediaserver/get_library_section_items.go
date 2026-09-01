package routes_ms

import (
	"aura/cache"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type GetLibrarySectionItems_Response struct {
	MediaItems        []models.MediaItem `json:"media_items"`
	TotalItems        int                `json:"total_items"`
	HasUpdatedAt      bool               `json:"has_updated_at"`
	HasEpisodeAddedAt bool               `json:"has_episode_added_at"`
}

type libraryItemsQuery struct {
	LibraryTitles []string
	SearchTitle   string
	SearchLibrary string
	SearchID      string
	SearchYear    int
	FilterInDB    string
	FilterIgnored string
	FilterHasSets string
	SortOption    string
	SortOrder     string
	PageNumber    int
	ItemsPerPage  int
}

type libraryItemsPage struct {
	Items             []models.MediaItem
	TotalItems        int
	HasUpdatedAt      bool
	HasEpisodeAddedAt bool
}

var refreshLibraryCache = mediaserver.GetAllLibrarySectionsAndItems

// GetLibrarySectionItems godoc
// @Summary      Get Library Items
// @Description  Retrieve one filtered, sorted page from the warmed media-server library cache.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        library_titles query string false "Library Title (repeat for multiple)"
// @Param        search_title query string false "Title search"
// @Param        search_library query string false "Library search"
// @Param        search_id query string false "Exact TMDB ID search"
// @Param        search_year query int false "Release year"
// @Param        filter_in_db query string false "Database filter (inDB or notInDB)"
// @Param        filter_ignored query string false "Ignore status or mode"
// @Param        filter_has_sets query string false "MediUX set availability"
// @Param        sort_option query string false "Sort option"
// @Param        sort_order query string false "Sort order (asc or desc)"
// @Param        page_number query int false "Page number"
// @Param        items_per_page query int false "Items per page (maximum 1000)"
// @Param        refresh query bool false "Refresh server cache before reading"
// @Security     BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetLibrarySectionItems_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/library/items [get]
func GetLibrarySectionItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Library Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	forceRefresh := strings.EqualFold(r.URL.Query().Get("refresh"), "true")
	if (forceRefresh || cache.LibraryStore.GetLastFullUpdate() == 0) && !refreshLibraryCache(ctx, forceRefresh) {
		logAction.SetError("Failed to refresh library items", "Retry after the media server is available", nil)
		httpx.SendResponse(w, ld, GetLibrarySectionItems_Response{})
		return
	}

	query := libraryItemsQuery{
		SearchTitle:   r.URL.Query().Get("search_title"),
		SearchLibrary: r.URL.Query().Get("search_library"),
		SearchID:      r.URL.Query().Get("search_id"),
		FilterInDB:    r.URL.Query().Get("filter_in_db"),
		FilterIgnored: r.URL.Query().Get("filter_ignored"),
		FilterHasSets: r.URL.Query().Get("filter_has_sets"),
		SortOption:    r.URL.Query().Get("sort_option"),
		SortOrder:     r.URL.Query().Get("sort_order"),
		SearchYear:    positiveInt(r.URL.Query().Get("search_year"), 0),
		PageNumber:    positiveInt(r.URL.Query().Get("page_number"), 1),
		ItemsPerPage:  positiveInt(r.URL.Query().Get("items_per_page"), 20),
	}
	query.LibraryTitles = r.URL.Query()["library_titles[]"]
	if len(query.LibraryTitles) == 0 {
		query.LibraryTitles = r.URL.Query()["library_titles"]
	}
	if query.ItemsPerPage > 1000 {
		query.ItemsPerPage = 1000
	}

	page := filterSortAndPaginateLibraryItems(cache.LibraryStore.GetAllMediaItems(), query)
	response := GetLibrarySectionItems_Response{
		MediaItems:        page.Items,
		TotalItems:        page.TotalItems,
		HasUpdatedAt:      page.HasUpdatedAt,
		HasEpisodeAddedAt: page.HasEpisodeAddedAt,
	}
	httpx.SendResponse(w, ld, response)
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func filterSortAndPaginateLibraryItems(items []models.MediaItem, query libraryItemsQuery) libraryItemsPage {
	selectedLibraries := make(map[string]struct{}, len(query.LibraryTitles))
	for _, title := range query.LibraryTitles {
		selectedLibraries[title] = struct{}{}
	}

	filtered := make([]models.MediaItem, 0, len(items))
	for _, item := range items {
		if len(selectedLibraries) > 0 {
			if _, selected := selectedLibraries[item.LibraryTitle]; !selected {
				continue
			}
		}
		if query.SearchYear > 0 && item.Year != query.SearchYear {
			continue
		}
		if query.SearchLibrary != "" && !strings.Contains(normalizeLibrarySearch(item.LibraryTitle), normalizeLibrarySearch(query.SearchLibrary)) {
			continue
		}
		if query.SearchID != "" && item.TMDB_ID != query.SearchID {
			continue
		}
		if !titleMatches(item.Title, query.SearchTitle) {
			continue
		}
		if query.FilterInDB == "notInDB" && len(item.DBSavedSets) > 0 {
			continue
		}
		if query.FilterInDB == "inDB" && len(item.DBSavedSets) == 0 {
			continue
		}
		if !ignoredMatches(item, query.FilterIgnored) {
			continue
		}
		if query.FilterHasSets == "hasSetsAvailable" && !item.HasMediuxSets {
			continue
		}
		if query.FilterHasSets == "noSetsAvailable" && item.HasMediuxSets {
			continue
		}
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		cmp := compareLibraryItems(filtered[i], filtered[j], query.SortOption)
		if strings.EqualFold(query.SortOrder, "desc") {
			return cmp > 0
		}
		return cmp < 0
	})

	page := libraryItemsPage{TotalItems: len(filtered), Items: []models.MediaItem{}}
	for _, item := range filtered {
		page.HasUpdatedAt = page.HasUpdatedAt || item.UpdatedAt > 0
		page.HasEpisodeAddedAt = page.HasEpisodeAddedAt || item.LatestEpisodeAddedAt > 0
	}
	start := (query.PageNumber - 1) * query.ItemsPerPage
	if start >= len(filtered) {
		return page
	}
	end := min(start+query.ItemsPerPage, len(filtered))
	page.Items = filtered[start:end]
	return page
}

func compareLibraryItems(a, b models.MediaItem, option string) int {
	switch option {
	case "title":
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	case "dateUpdated":
		return intCompare(a.UpdatedAt, b.UpdatedAt)
	case "dateReleased":
		return intCompare(a.ReleasedAt, b.ReleasedAt)
	case "newEpisodeAdded":
		aDate, bDate := a.LatestEpisodeAddedAt, b.LatestEpisodeAddedAt
		if aDate == 0 {
			aDate = a.AddedAt
		}
		if bDate == 0 {
			bDate = b.AddedAt
		}
		return intCompare(aDate, bDate)
	default:
		return intCompare(a.AddedAt, b.AddedAt)
	}
}

func intCompare(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func ignoredMatches(item models.MediaItem, filter string) bool {
	switch filter {
	case "ignored":
		return item.IgnoredInDB
	case "not_ignored":
		return !item.IgnoredInDB
	case "always", "until-set-available", "until-new-set-available":
		return item.IgnoredInDB && item.IgnoredMode == filter
	default:
		return true
	}
}

func titleMatches(title, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	normalizedTitle := normalizeLibrarySearch(title)
	quotePairs := [][2]string{{`"`, `"`}, {`'`, `'`}, {`‘`, `’`}, {`“`, `”`}}
	for _, pair := range quotePairs {
		if strings.HasPrefix(query, pair[0]) && strings.HasSuffix(query, pair[1]) {
			unquoted := strings.TrimSuffix(strings.TrimPrefix(query, pair[0]), pair[1])
			return normalizedTitle == normalizeLibrarySearch(unquoted)
		}
	}
	for _, word := range strings.Fields(normalizeLibrarySearch(query)) {
		if !strings.Contains(normalizedTitle, word) {
			return false
		}
	}
	return true
}

func normalizeLibrarySearch(value string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(value) {
		if r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || unicode.IsSpace(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}
