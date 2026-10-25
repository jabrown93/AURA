package library

import (
	"aura/cache"
	"aura/models"
	"slices"
	"testing"
)

func testStore(t *testing.T) *cache.MediaServerLibraryCache {
	t.Helper()
	store := cache.Cache_NewLibraryCache()
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
		MediaItems: []models.MediaItem{
			{RatingKey: "1", TMDB_ID: "603", Type: "movie", Title: "The Matrix", Year: 1999, LibraryTitle: "Movies", AddedAt: 300, HasMediuxSets: true},
			{RatingKey: "2", TMDB_ID: "604", Type: "movie", Title: "The Matrix Reloaded", Year: 2003, LibraryTitle: "Movies", AddedAt: 100,
				DBSavedSets: []models.DBSavedSet{{ID: "set"}}},
			{RatingKey: "3", TMDB_ID: "605", Type: "movie", Title: "Amélie", Year: 2001, LibraryTitle: "Movies", AddedAt: 200,
				IgnoredInDB: true, IgnoredMode: "always"},
			{RatingKey: "4", TMDB_ID: "603", Type: "movie", Title: "The Matrix (copy)", LibraryTitle: "Movies"}, // Same item and edition as 1
			{RatingKey: "5", TMDB_ID: "603", Type: "movie", Title: "The Matrix", Edition: "4K", LibraryTitle: "Movies", AddedAt: 50},
		},
	})
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Series", Type: "show"},
		MediaItems: []models.MediaItem{
			{RatingKey: "10", TMDB_ID: "1396", Type: "show", Title: "Breaking Bad", Year: 2008, LibraryTitle: "Series", AddedAt: 10, LatestEpisodeAddedAt: 400},
		},
	})
	return store
}

func keys(result Result) []string {
	out := []string{}
	for _, item := range result.Items {
		out = append(out, item.RatingKey)
	}
	return out
}

func TestFindFiltersAndSorts(t *testing.T) {
	store := testStore(t)
	tests := []struct {
		name  string
		query Query
		want  []string
	}{
		{"newest first", Query{Sort: "dateAdded", Order: "desc"}, []string{"1", "3", "2", "5", "10"}},
		{"title", Query{Sort: "title", Order: "asc"}, []string{"3", "10", "1", "5", "2"}},
		{"latest episode, movies by date added", Query{Sort: "newEpisodeAdded", Order: "desc"}, []string{"10", "1", "3", "2", "5"}},
		{"one library", Query{Libraries: []string{"Series"}}, []string{"10"}},
		{"saved", Query{InDB: "inDB"}, []string{"2"}},
		{"not saved", Query{InDB: "notInDB", Sort: "dateAdded", Order: "asc"}, []string{"10", "5", "3", "1"}},
		{"ignored always", Query{Ignored: "always"}, []string{"3"}},
		{"sets available", Query{SetsAvailable: "hasSetsAvailable"}, []string{"1"}},
		{"search words", Query{Search: "matrix reloaded"}, []string{"2"}},
		{"search with year", Query{Search: "matrix Y:1999:"}, []string{"1"}},
		{"search exact title", Query{Search: `"the matrix"`, Sort: "title", Order: "asc"}, []string{"1", "5"}},
		{"search library", Query{Search: "L:series:"}, []string{"10"}},
		{"search ID", Query{Search: "ID:604:"}, []string{"2"}},
		{"search ignores punctuation", Query{Search: "amelie"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.query.PerPage = 50
			if got := keys(Find(store, tt.query)); !slices.Equal(got, tt.want) {
				t.Errorf("Find = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindPagesAndTotals(t *testing.T) {
	result := Find(testStore(t), Query{Sort: "dateAdded", Order: "desc", Page: 2, PerPage: 2})
	if !slices.Equal(keys(result), []string{"2", "5"}) || result.Total != 5 || result.TotalPages != 3 || result.Page != 2 {
		t.Errorf("page 2 = %v, total %d, pages %d, page %d", keys(result), result.Total, result.TotalPages, result.Page)
	}
	// A page past the end shows the last page
	if result := Find(testStore(t), Query{Page: 9, PerPage: 2}); result.Page != 3 {
		t.Errorf("page past the end = %d, want 3", result.Page)
	}

	want := []LibrarySummary{{Title: "Movies", Type: "movie", Count: 4}, {Title: "Series", Type: "show", Count: 1}}
	if !slices.Equal(result.Libraries, want) {
		t.Errorf("libraries = %+v, want %+v (the duplicate isn't counted)", result.Libraries, want)
	}
	if !result.HasEpisodeAddedAt || result.HasUpdatedAt {
		t.Errorf("has episode added = %v, has updated = %v, want true and false", result.HasEpisodeAddedAt, result.HasUpdatedAt)
	}
}

func TestNeighbours(t *testing.T) {
	store := testStore(t)
	q := Query{Sort: "dateAdded", Order: "desc"}
	tests := []struct {
		name, library, ratingKey string
		previous, next           string
	}{
		{"middle", "Movies", "3", "1", "2"},
		{"first goes back to the last", "Movies", "1", "10", "3"},
		{"last goes on to the first", "Series", "10", "5", "1"},
		{"not in the list", "Movies", "4", "", ""},
	}
	key := func(item *Item) string {
		if item == nil {
			return ""
		}
		return item.RatingKey
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous, next := Neighbours(store, q, tt.library, tt.ratingKey)
			if key(previous) != tt.previous || key(next) != tt.next {
				t.Errorf("Neighbours = %q, %q, want %q, %q", key(previous), key(next), tt.previous, tt.next)
			}
		})
	}
}

func TestInOtherLibraries(t *testing.T) {
	store := testStore(t)
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "4K Movies", Type: "movie"},
		MediaItems:         []models.MediaItem{{RatingKey: "40", TMDB_ID: "603", Type: "movie", Title: "The Matrix", LibraryTitle: "4K Movies"}},
	})
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Anime", Type: "show"},
		MediaItems:         []models.MediaItem{{RatingKey: "50", TMDB_ID: "603", Type: "show", Title: "Not The Matrix", LibraryTitle: "Anime"}},
	})

	found := InOtherLibraries(store, "Movies", "603")
	if len(found) != 1 || found[0].RatingKey != "40" {
		t.Errorf("InOtherLibraries = %+v, want only the 4K Movies item", found)
	}
	if found := InOtherLibraries(store, "Movies", ""); len(found) != 0 {
		t.Errorf("no TMDB ID = %+v, want none", found)
	}
}
