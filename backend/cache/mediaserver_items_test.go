package cache

import (
	"aura/models"
	"testing"
)

// On a cache hit AddNewItemToDB uses the narrow writer, because the cached entry is
// media-server-owned and richer than the request's copy that UpdateMediaItem would
// write over wholesale.
func TestUpdateMediaItemDBSavedSetsPreservesOtherFields(t *testing.T) {
	c := Cache_NewLibraryCache()
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", Title: "Fight Club", RatingKey: "1234", Year: 1999},
		},
	})

	stale := models.MediaItem{TMDB_ID: "550"}
	c.UpdateMediaItemDBSavedSets("Movies", &stale, []models.DBSavedSet{{ID: "set-1"}})

	section, ok := c.GetSectionByTitle("Movies")
	if !ok {
		t.Fatal("section 'Movies' missing from cache")
	}
	got := section.MediaItems[0]
	if len(got.DBSavedSets) != 1 || got.DBSavedSets[0].ID != "set-1" {
		t.Errorf("DBSavedSets = %+v, want exactly one set with ID 'set-1'", got.DBSavedSets)
	}
	if got.Title != "Fight Club" || got.RatingKey != "1234" || got.Year != 1999 {
		t.Errorf("cached item was clobbered: %+v", got)
	}
}

// On a cache miss AddNewItemToDB falls back to UpdateMediaItem, since the narrow writer
// no-ops on an absent item and would strand the saved sets until the next full refresh.
func TestUpdateSectionReplacesSnapshotByTMDBIdentity(t *testing.T) {
	c := newLibraryCache(0)
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", RatingKey: "old-key", Title: "Fight Club", UpdatedAt: 200},
			{TMDB_ID: "680", RatingKey: "removed-key", Title: "Pulp Fiction", UpdatedAt: 100},
		},
	})

	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", RatingKey: "new-key", Title: "Fight Club refreshed", UpdatedAt: 150},
		},
	})

	section, ok := c.GetSectionByTitle("Movies")
	if !ok || len(section.MediaItems) != 1 {
		t.Fatalf("section = %+v, found = %v; want one-item replacement", section, ok)
	}
	got := section.MediaItems[0]
	if got.RatingKey != "new-key" || got.UpdatedAt != 200 {
		t.Fatalf("refreshed item = %+v, want new rating key and preserved version 200", got)
	}
	if _, found := c.GetMediaItemByRatingKey("old-key"); found {
		t.Fatal("stale rating-key duplicate survived replacement")
	}
}

func TestUpdateSectionFallsBackToRatingKeyWithoutTMDBID(t *testing.T) {
	c := newLibraryCache(0)
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 200}},
	})
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", Title: "refreshed", UpdatedAt: 100}},
	})

	got, ok := c.GetMediaItemByRatingKey("show-1")
	if !ok || got.Title != "refreshed" || got.UpdatedAt != 200 {
		t.Fatalf("fallback identity item = %+v, found = %v", got, ok)
	}
}

func TestReplaceAllSectionsPrunesAndCopiesSnapshots(t *testing.T) {
	c := newLibraryCache(0)
	c.UpdateSection(&models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Removed"}})
	sections := []*models.LibrarySection{{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Paths: []string{"/movies"}},
		MediaItems:         []models.MediaItem{{TMDB_ID: "550", RatingKey: "movie-1"}},
	}}

	c.ReplaceAllSections(sections, 123)
	sections[0].Paths[0] = "/mutated"
	sections[0].MediaItems[0].Title = "mutated"

	if _, found := c.GetSectionByTitle("Removed"); found {
		t.Fatal("de-configured section survived successful replacement")
	}
	got, found := c.GetSectionByTitle("Movies")
	if !found || got.Paths[0] != "/movies" || got.MediaItems[0].Title == "mutated" {
		t.Fatalf("cache retained caller-owned snapshot: %+v", got)
	}
	if c.GetLastFullUpdate() != 123 {
		t.Fatalf("LastFullUpdate = %d, want 123", c.GetLastFullUpdate())
	}
}

func TestSetIgnoredMutatesOnlyMatchingCachedItem(t *testing.T) {
	c := newLibraryCache(0)
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", DBSavedSets: []models.DBSavedSet{{ID: "set-1"}}},
			{TMDB_ID: "680"},
		},
	})

	if !c.SetIgnored("Movies", "550", true, "always") {
		t.Fatal("SetIgnored() did not find TMDB 550")
	}
	ignored, _ := c.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if !ignored.IgnoredInDB || ignored.IgnoredMode != "always" || len(ignored.DBSavedSets) != 1 {
		t.Fatalf("ignored item = %+v, want independent saved-set state", ignored)
	}
	if !c.SetIgnored("Movies", "550", false, "") {
		t.Fatal("SetIgnored() did not unignore TMDB 550")
	}
	unignored, _ := c.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if unignored.IgnoredInDB || unignored.IgnoredMode != "" || len(unignored.DBSavedSets) != 1 {
		t.Fatalf("unignored item = %+v, want saved sets retained", unignored)
	}
}

func TestCacheGettersDoNotExposeMutableInterior(t *testing.T) {
	c := newLibraryCache(0)
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV", Paths: []string{"/tv"}},
		MediaItems: []models.MediaItem{{
			TMDB_ID: "1", RatingKey: "show-1",
			Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{Title: "Season 1"}}},
		}},
	})

	section, _ := c.GetSectionByTitle("TV")
	section.Paths[0] = "/changed"
	section.MediaItems[0].Series.Seasons[0].Title = "changed"
	item, _ := c.GetMediaItemByRatingKey("show-1")
	item.Series.Seasons[0].Title = "changed again"

	got, _ := c.GetSectionByTitle("TV")
	if got.Paths[0] != "/tv" || got.MediaItems[0].Series.Seasons[0].Title != "Season 1" {
		t.Fatalf("getter mutation reached cache: %+v", got)
	}
}

func TestUpdateMediaItemInsertsMissingItemWithDBSavedSets(t *testing.T) {
	c := Cache_NewLibraryCache()
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", Title: "Fight Club"},
		},
	})

	if _, found := c.GetMediaItemFromSectionByTMDBID("Movies", "680"); found {
		t.Fatal("TMDB 680 should not be cached yet")
	}

	added := models.MediaItem{
		TMDB_ID:      "680",
		Title:        "Pulp Fiction",
		LibraryTitle: "Movies",
		DBSavedSets:  []models.DBSavedSet{{ID: "set-2"}},
	}
	c.UpdateMediaItem("Movies", &added)

	section, ok := c.GetSectionByTitle("Movies")
	if !ok {
		t.Fatal("section 'Movies' missing from cache")
	}
	if len(section.MediaItems) != 2 || section.TotalSize != 2 {
		t.Fatalf("want 2 items and TotalSize 2, got %d items and TotalSize %d",
			len(section.MediaItems), section.TotalSize)
	}
	got, found := c.GetMediaItemFromSectionByTMDBID("Movies", "680")
	if !found {
		t.Fatal("inserted item not retrievable by TMDB ID")
	}
	if len(got.DBSavedSets) != 1 || got.DBSavedSets[0].ID != "set-2" {
		t.Errorf("DBSavedSets = %+v, want exactly one set with ID 'set-2'", got.DBSavedSets)
	}
	if section.MediaItems[0].Title != "Fight Club" {
		t.Errorf("pre-existing item disturbed: %+v", section.MediaItems[0])
	}
}
