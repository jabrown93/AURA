package cache

import (
	"aura/models"
	"testing"
)

// AddNewItemToDB refreshes only DBSavedSets from a detached goroutine and passes the
// request's partial MediaItem. UpdateMediaItem would replace the cached item wholesale,
// so the narrow writer must leave every media-server-owned field untouched.
func TestUpdateMediaItemDBSavedSetsPreservesOtherFields(t *testing.T) {
	c := Cache_NewLibraryCache()
	c.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "550", Title: "Fight Club", RatingKey: "1234", Year: 1999},
		},
	})

	partial := models.MediaItem{TMDB_ID: "550"}
	c.UpdateMediaItemDBSavedSets("Movies", &partial, []models.DBSavedSet{{ID: "set-1"}})

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
