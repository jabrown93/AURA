package creator

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func set(id, title string, updated time.Time, items ...string) models.SetRef {
	return models.SetRef{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, Title: title, UserCreated: "creator", DateUpdated: updated}}, ItemIDs: items}
}

// setupCreator has a Series library with shows 1 and 2, a Movies library with movies 10 and 11, and a creator with sets for some of them
func setupCreator(t *testing.T) (*cache.MediaServerLibraryCache, *int) {
	t.Helper()
	store := cache.Cache_NewLibraryCache()
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Series", Type: "show"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "1", Title: "Breaking Bad", RatingKey: "s1", LibraryTitle: "Series", DBSavedSets: []models.DBSavedSet{{ID: "showA"}}},
			{TMDB_ID: "2", Title: "Better Call Saul", RatingKey: "s2", LibraryTitle: "Series"},
		},
	})
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "10", Title: "The Matrix", RatingKey: "m10", LibraryTitle: "Movies", DBSavedSets: []models.DBSavedSet{{ID: "other", UserCreated: "someone"}}},
			{TMDB_ID: "11", Title: "The Matrix Reloaded", RatingKey: "m11", LibraryTitle: "Movies"},
		},
	})

	day := func(d int) time.Time { return time.Date(2025, 1, d, 0, 0, 0, 0, time.UTC) }
	sets := models.CreatorSetsResponse{
		ShowSets: []models.SetRef{
			set("showA", "Breaking Bad Set", day(1), "1"),
			set("showB", "Better Call Saul Set", day(3), "2"),
			set("showC", "Not In Library", day(5), "99"),
		},
		MovieSets:      []models.SetRef{set("movieM", "Matrix Poster", day(2), "10")},
		CollectionSets: []models.SetRef{set("collectionC", "Matrix Collection", day(4), "10", "11")},
		Boxsets: []models.BoxsetRef{{
			BaseSetInfo: models.BaseSetInfo{ID: "box", Title: "Everything"},
			SetIDs:      map[string][]string{"show": {"showA", "showC"}, "movie": {"movieM"}},
		}},
		IncludedItems: map[string]models.IncludedItem{
			"1": {}, "2": {}, "10": {}, "11": {}, "99": {},
		},
	}
	for id, item := range sets.IncludedItems {
		item.MediuxInfo.TMDB_ID = id
		sets.IncludedItems[id] = item
	}
	// MediUX gives each set its type
	for setType, list := range map[string][]models.SetRef{"show": sets.ShowSets, "movie": sets.MovieSets, "collection": sets.CollectionSets} {
		for i := range list {
			list[i].Type = setType
		}
	}

	calls := new(int)
	previous := getAllUserSets
	t.Cleanup(func() {
		getAllUserSets = previous
		setsMu.Lock()
		setsCache = map[string]savedSets{}
		setsMu.Unlock()
	})
	getAllUserSets = func(ctx context.Context, username string) (models.CreatorSetsResponse, logging.LogErrorInfo) {
		*calls++
		return sets, logging.LogErrorInfo{}
	}
	return store, calls
}

func setIDs(result Result) []string {
	ids := []string{}
	for _, s := range result.Sets {
		ids = append(ids, s.ID)
	}
	for _, b := range result.Boxsets {
		ids = append(ids, b.ID)
		for _, s := range b.Sets {
			ids = append(ids, "  "+s.ID)
		}
	}
	return ids
}

func TestFindMatchesSetsToTheLibrary(t *testing.T) {
	store, _ := setupCreator(t)
	tests := []struct {
		name  string
		query Query
		want  []string
	}{
		{"show sets in a show library, newest first", Query{Library: "Series", Tab: TabShowSets}, []string{"showB", "showA"}},
		{"by title", Query{Library: "Series", Tab: TabShowSets, Sort: "title", Order: "asc"}, []string{"showB", "showA"}},
		{"movie sets", Query{Library: "Movies", Tab: TabMovieSets}, []string{"movieM"}},
		{"collection sets", Query{Library: "Movies", Tab: TabCollectionSets}, []string{"collectionC"}},
		{"boxset keeps the show sets in the library", Query{Library: "Series", Tab: TabBoxSets}, []string{"box", "  showA"}},
		{"boxset keeps the movie sets in the library", Query{Library: "Movies", Tab: TabBoxSets}, []string{"box", "  movieM"}},
		{"saved with this set", Query{Library: "Series", Tab: TabShowSets, InDB: "inDB"}, []string{"showA"}},
		{"not saved", Query{Library: "Series", Tab: TabShowSets, InDB: "notInDB"}, []string{"showB"}},
		{"saved with another creator's set", Query{Library: "Movies", Tab: TabMovieSets, InDB: "otherSetInDB"}, []string{"movieM"}},
		{"search by item title", Query{Library: "Series", Tab: TabShowSets, Search: "saul"}, []string{"showB"}},
		{"search and filter together", Query{Library: "Series", Tab: TabShowSets, Search: "b", InDB: "notInDB"}, []string{"showB"}},
		{"boxset search by an item in it", Query{Library: "Series", Tab: TabBoxSets, Search: "breaking"}, []string{"box", "  showA"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.query.Username = "creator"
			result, Err := Find(testContext(), store, tt.query)
			if Err.Message != "" {
				t.Fatalf("Find: %s", Err.Message)
			}
			if got := setIDs(result); !slices.Equal(got, tt.want) {
				t.Errorf("Find = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindCountsAndIncludedItems(t *testing.T) {
	store, calls := setupCreator(t)

	result, _ := Find(testContext(), store, Query{Username: "creator", Library: "Movies", Tab: TabMovieSets})
	if result.Counts != (TabCounts{MovieSets: 1, CollectionSets: 1, BoxSets: 1}) {
		t.Errorf("counts = %+v", result.Counts)
	}
	if result.Totals != (TabCounts{ShowSets: 3, MovieSets: 1, CollectionSets: 1, BoxSets: 1}) {
		t.Errorf("totals = %+v", result.Totals)
	}
	// Only the items of the sets on the page, with the library item
	if len(result.IncludedItems) != 1 || result.IncludedItems["10"].MediaItem.RatingKey != "m10" {
		t.Errorf("included items = %+v", result.IncludedItems)
	}
	want := []LibrarySummary{{Title: "Movies", Type: "movie", Sets: 2}, {Title: "Series", Type: "show", Sets: 2}}
	if !slices.Equal(result.Libraries, want) {
		t.Errorf("libraries = %+v, want %+v", result.Libraries, want)
	}

	// Without a library, only the libraries are listed
	if result, _ := Find(testContext(), store, Query{Username: "creator"}); len(result.Sets) != 0 || len(result.Libraries) != 2 {
		t.Errorf("no library = %d sets, %d libraries", len(result.Sets), len(result.Libraries))
	}

	// MediUX is only asked again when refreshing
	if *calls != 1 {
		t.Errorf("MediUX was asked %d times, want once", *calls)
	}
	_, _ = Find(testContext(), store, Query{Username: "creator", Refresh: true})
	if *calls != 2 {
		t.Errorf("a refresh should ask MediUX again, asked %d times", *calls)
	}
}

func TestFindMixedLibrary(t *testing.T) {
	_, _ = setupCreator(t)
	// A mixed library has shows and movies, movie 2 shares its TMDB ID with the show Better Call Saul, which isn't in the library
	store := cache.Cache_NewLibraryCache()
	store.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Everything", Type: "mixed"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "1", Type: "show", Title: "Breaking Bad", RatingKey: "s1", LibraryTitle: "Everything", DBSavedSets: []models.DBSavedSet{{ID: "showA"}}},
			{TMDB_ID: "2", Type: "movie", Title: "Ariel", RatingKey: "m2", LibraryTitle: "Everything"},
			{TMDB_ID: "10", Type: "movie", Title: "The Matrix", RatingKey: "m10", LibraryTitle: "Everything"},
		},
	})

	tests := []struct {
		tab  string
		want []string
	}{
		{TabShowSets, []string{"showA"}},
		{TabMovieSets, []string{"movieM"}},
		{TabCollectionSets, []string{"collectionC"}},
		{TabBoxSets, []string{"box", "  showA", "  movieM"}},
	}
	for _, tt := range tests {
		result, Err := Find(testContext(), store, Query{Username: "creator", Library: "Everything", Tab: tt.tab})
		if Err.Message != "" {
			t.Fatalf("Find: %s", Err.Message)
		}
		if got := setIDs(result); !slices.Equal(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.tab, got, tt.want)
		}
		if tt.tab == TabShowSets && result.IncludedItems["1"].MediaItem.RatingKey != "s1" {
			t.Errorf("included items = %+v, want the show", result.IncludedItems)
		}
	}

	result, _ := Find(testContext(), store, Query{Username: "creator"})
	want := []LibrarySummary{{Title: "Everything", Type: "mixed", Sets: 3}}
	if !slices.Equal(result.Libraries, want) {
		t.Errorf("libraries = %+v, want %+v", result.Libraries, want)
	}
}
