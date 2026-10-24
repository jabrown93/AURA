package library

import (
	"aura/cache"
	"aura/models"
	"cmp"
	"slices"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Item is a media item with only what the Home page shows, sorts and filters on
type Item struct {
	RatingKey            string              `json:"rating_key"`
	TMDB_ID              string              `json:"tmdb_id"`
	Type                 string              `json:"type"`
	Title                string              `json:"title"`
	Year                 int                 `json:"year"`
	LibraryTitle         string              `json:"library_title"`
	Edition              string              `json:"edition"`
	DBSavedSets          []models.DBSavedSet `json:"db_saved_sets"`
	IgnoredInDB          bool                `json:"ignored_in_db"`
	IgnoredMode          string              `json:"ignored_mode"`
	HasMediuxSets        bool                `json:"has_mediux_sets"`
	UpdatedAt            int64               `json:"updated_at"`
	AddedAt              int64               `json:"added_at"`
	ReleasedAt           int64               `json:"released_at"`
	LatestEpisodeAddedAt int64               `json:"latest_episode_added_at"`
}

// Query is what the Home page asks for: which items, in which order, and which page
type Query struct {
	Libraries     []string // Only these libraries. Empty is every library.
	Search        string   // Same as the search bar, e.g. "matrix Y:1999:"
	InDB          string   // "inDB", "notInDB" or empty
	Ignored       string   // "ignored", "not_ignored", an ignore mode, or empty
	SetsAvailable string   // "hasSetsAvailable", "noSetsAvailable" or empty
	Sort          string   // "title", "dateUpdated", "dateAdded", "dateReleased" or "newEpisodeAdded"
	Order         string   // "asc" or "desc"
	Page          int      // From 1
	PerPage       int
}

// LibrarySummary is a library and how many items it has
type LibrarySummary struct {
	Title string `json:"title"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Result is one page of items, with the totals for every page
type Result struct {
	Items             []Item           `json:"items"`
	Total             int              `json:"total"` // Items on every page
	Page              int              `json:"page"`
	PerPage           int              `json:"per_page"`
	TotalPages        int              `json:"total_pages"`
	Libraries         []LibrarySummary `json:"libraries"`
	HasUpdatedAt      bool             `json:"has_updated_at"`       // Whether the "Date Updated" sort is useful
	HasEpisodeAddedAt bool             `json:"has_episode_added_at"` // Whether the "Latest Episode Added" sort is useful
	LastFullUpdate    int64            `json:"last_full_update"`
}

const defaultPerPage = 20

// Find returns one page of the library, filtered, searched and sorted like the Home page
func Find(store *cache.MediaServerLibraryCache, q Query) Result {
	result := Result{Items: []Item{}, Libraries: []LibrarySummary{}, LastFullUpdate: store.LastFullUpdate}
	matched := match(store, q, &result)

	for _, item := range matched {
		result.HasUpdatedAt = result.HasUpdatedAt || item.UpdatedAt > 0
		result.HasEpisodeAddedAt = result.HasEpisodeAddedAt || item.LatestEpisodeAddedAt > 0
	}

	// The page, kept within the pages there are
	result.PerPage = q.PerPage
	if result.PerPage <= 0 {
		result.PerPage = defaultPerPage
	}
	result.Total = len(matched)
	result.TotalPages = max(1, (result.Total+result.PerPage-1)/result.PerPage)
	result.Page = min(max(q.Page, 1), result.TotalPages)
	start := (result.Page - 1) * result.PerPage
	end := min(start+result.PerPage, result.Total)
	if start < end {
		result.Items = matched[start:end]
	}
	return result
}

// match returns the items that match the filters and search, in the Home page's order
func match(store *cache.MediaServerLibraryCache, q Query, result *Result) []Item {
	items := snapshot(store, result)
	s := parseSearch(q.Search)
	matched := items[:0]
	for _, item := range items {
		if matchesFilters(item, q) && s.matches(item) {
			matched = append(matched, item)
		}
	}
	sortItems(matched, q.Sort, q.Order)
	return matched
}

// Neighbours returns the items before and after an item in the Home page's order, wrapping around
func Neighbours(store *cache.MediaServerLibraryCache, q Query, libraryTitle, ratingKey string) (previous, next *Item) {
	matched := match(store, q, &Result{})
	i := slices.IndexFunc(matched, func(item Item) bool {
		return item.LibraryTitle == libraryTitle && item.RatingKey == ratingKey
	})
	if i == -1 || len(matched) < 2 {
		return nil, nil
	}
	n := len(matched)
	return &matched[(i-1+n)%n], &matched[(i+1)%n]
}

// InOtherLibraries returns the item from each other library of the same type that has it
func InOtherLibraries(store *cache.MediaServerLibraryCache, libraryTitle, tmdbID string) []models.MediaItem {
	found := []models.MediaItem{}
	if tmdbID == "" {
		return found
	}
	itemType := ""
	store.ForEachMediaItem(func(section models.LibrarySectionBase, m *models.MediaItem) {
		if section.Title == libraryTitle && m.TMDB_ID == tmdbID {
			itemType = section.Type
		}
	})
	seen := map[string]bool{}
	store.ForEachMediaItem(func(section models.LibrarySectionBase, m *models.MediaItem) {
		if section.Title == libraryTitle || section.Type != itemType || seen[section.Title] || m.TMDB_ID != tmdbID {
			return
		}
		seen[section.Title] = true
		found = append(found, *m)
	})
	return found
}

// snapshot copies the items out of the cache, once per item and edition in each library
func snapshot(store *cache.MediaServerLibraryCache, result *Result) []Item {
	items := []Item{}
	seen := map[string]bool{}
	counts := map[string]int{}
	store.ForEachMediaItem(func(section models.LibrarySectionBase, m *models.MediaItem) {
		if _, ok := counts[section.Title]; !ok {
			result.Libraries = append(result.Libraries, LibrarySummary{Title: section.Title, Type: section.Type})
			counts[section.Title] = 0
		}
		if m.TMDB_ID != "" {
			key := section.Title + "|" + m.TMDB_ID + "|" + m.Edition
			if seen[key] {
				return
			}
			seen[key] = true
		}
		counts[section.Title]++
		items = append(items, Item{
			RatingKey: m.RatingKey, TMDB_ID: m.TMDB_ID, Type: m.Type, Title: m.Title, Year: m.Year,
			LibraryTitle: m.LibraryTitle, Edition: m.Edition,
			DBSavedSets: m.DBSavedSets, IgnoredInDB: m.IgnoredInDB, IgnoredMode: m.IgnoredMode, HasMediuxSets: m.HasMediuxSets,
			UpdatedAt: m.UpdatedAt, AddedAt: m.AddedAt, ReleasedAt: m.ReleasedAt, LatestEpisodeAddedAt: m.LatestEpisodeAddedAt,
		})
	})
	for i := range result.Libraries {
		result.Libraries[i].Count = counts[result.Libraries[i].Title]
	}
	return items
}

func matchesFilters(item Item, q Query) bool {
	if len(q.Libraries) > 0 && !slices.Contains(q.Libraries, item.LibraryTitle) {
		return false
	}

	saved := len(item.DBSavedSets) > 0
	switch q.InDB {
	case "inDB":
		if !saved {
			return false
		}
	case "notInDB":
		if saved {
			return false
		}
	}

	switch q.Ignored {
	case "ignored":
		if !item.IgnoredInDB {
			return false
		}
	case "not_ignored":
		if item.IgnoredInDB {
			return false
		}
	case "always", "until-set-available", "until-new-set-available":
		if !item.IgnoredInDB || item.IgnoredMode != q.Ignored {
			return false
		}
	}

	switch q.SetsAvailable {
	case "hasSetsAvailable":
		return item.HasMediuxSets
	case "noSetsAvailable":
		return !item.HasMediuxSets
	}
	return true
}

func sortItems(items []Item, sortBy, order string) {
	var compare func(a, b Item) int
	switch sortBy {
	case "title":
		collator := collate.New(language.English)
		compare = func(a, b Item) int { return collator.CompareString(a.Title, b.Title) }
	case "dateUpdated":
		compare = func(a, b Item) int { return cmp.Compare(a.UpdatedAt, b.UpdatedAt) }
	case "dateReleased":
		compare = func(a, b Item) int { return cmp.Compare(a.ReleasedAt, b.ReleasedAt) }
	case "newEpisodeAdded":
		compare = func(a, b Item) int { return cmp.Compare(latestEpisodeAdded(a), latestEpisodeAdded(b)) }
	default:
		compare = func(a, b Item) int { return cmp.Compare(a.AddedAt, b.AddedAt) }
	}
	if order == "desc" {
		ascending := compare
		compare = func(a, b Item) int { return ascending(b, a) }
	}
	slices.SortStableFunc(items, compare)
}

// latestEpisodeAdded is when a show's newest episode was added, or when the item was added for a movie
func latestEpisodeAdded(item Item) int64 {
	if item.LatestEpisodeAddedAt > 0 {
		return item.LatestEpisodeAddedAt
	}
	return item.AddedAt
}
