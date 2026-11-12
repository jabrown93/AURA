package database

import (
	"aura/cache"
	"aura/models"
	"testing"
)

func TestAutoIgnoreUpdatesLibraryCacheFromDatabase(t *testing.T) {
	for _, existingMode := range []string{"", "always", "until-new-set-available"} {
		t.Run("existing="+existingMode, func(t *testing.T) {
			db := openTestMediaDB(t)
			ctx := testContext()
			previousClient, previousStore := Client, cache.LibraryStore
			Client, cache.LibraryStore = db, cache.Cache_NewLibraryCache()
			t.Cleanup(func() { Client, cache.LibraryStore = previousClient, previousStore })

			seedMediaItem(t, db.conn, "1", "Movies", "Director", "100", "movie", 7)
			if existingMode != "" {
				exec(t, db.conn, `INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode, current_sets) VALUES ('1', 'Movies', 'Director', ?, 'set1')`, existingMode)
			}
			cache.LibraryStore.UpdateSection(&models.LibrarySection{
				LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
				MediaItems: []models.MediaItem{
					{TMDB_ID: "1", LibraryTitle: "Movies", Edition: "Director", DBSavedSets: []models.DBSavedSet{{ID: "7"}}},
					{TMDB_ID: "1", LibraryTitle: "Movies", Edition: "Theatrical"},
				},
			})

			if err := AutoIgnoreMediaItem(ctx, "1", "Movies", "Director"); err.Message != "" {
				t.Fatal(err.Message)
			}
			wantMode := existingMode
			if wantMode == "" {
				wantMode = "until-set-available"
			}
			item, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition("Movies", "1", "Director")
			if !item.IgnoredInDB || item.IgnoredMode != wantMode || len(item.DBSavedSets) != 0 {
				t.Fatalf("cached ignored=%v mode=%q sets=%v, want ignored with mode %q and no saved sets", item.IgnoredInDB, item.IgnoredMode, item.DBSavedSets, wantMode)
			}
			other, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition("Movies", "1", "Theatrical")
			if other.IgnoredInDB {
				t.Fatal("another edition was ignored")
			}
		})
	}
}

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
