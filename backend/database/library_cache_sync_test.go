package database

import (
	"aura/cache"
	"aura/models"
	"testing"
)

func TestDatabaseChangesUpdateTheLibraryCache(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	previousClient, previousStore := Client, cache.LibraryStore
	Client, cache.LibraryStore = db, cache.Cache_NewLibraryCache()
	t.Cleanup(func() { Client, cache.LibraryStore = previousClient, previousStore })

	seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 7)
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
		MediaItems:         []models.MediaItem{{TMDB_ID: "1", RatingKey: "100", LibraryTitle: "Movies", Title: "Title"}},
	})
	cached := func() models.MediaItem {
		item, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition("Movies", "1", "")
		return *item
	}

	if Err := IgnoreMediaItem(ctx, "1", "Movies", "", "always", ""); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if item := cached(); !item.IgnoredInDB || item.IgnoredMode != "always" || len(item.DBSavedSets) != 0 {
		t.Errorf("after ignoring: ignored %v, mode %q, %d saved sets", item.IgnoredInDB, item.IgnoredMode, len(item.DBSavedSets))
	}

	if Err := StopIgnoringMediaItem(ctx, "1", "Movies", ""); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if item := cached(); item.IgnoredInDB || item.IgnoredMode != "" || len(item.DBSavedSets) != 1 {
		t.Errorf("after ignoring stopped: ignored %v, mode %q, %d saved sets", item.IgnoredInDB, item.IgnoredMode, len(item.DBSavedSets))
	}

	if Err := DeleteAllPosterSetsForMediaItem(ctx, "1", "Movies", ""); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if item := cached(); len(item.DBSavedSets) != 0 {
		t.Errorf("after deleting: %d saved sets, want 0", len(item.DBSavedSets))
	}
}
