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
