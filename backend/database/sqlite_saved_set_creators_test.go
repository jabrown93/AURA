package database

import (
	"aura/models"
	"testing"
)

func TestGetSavedSetCreators(t *testing.T) {
	db := openTestMediaDB(t)
	// Poster sets 1 and 2 are "user", see seedMediaItem
	seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 1, 2)
	seedMediaItem(t, db.conn, "2", "Movies", "", "101", "movie", 1, 3)
	exec(t, db.conn, `UPDATE PosterSets SET user = 'other' WHERE id = 3;`)
	exec(t, db.conn, `INSERT INTO PosterSets (id, set_id, type, title, user) VALUES (4, 4, 'movie', 'Not saved', 'nobody');`)

	creators, Err := db.GetSavedSetCreators(testContext())
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	// A set saved for two items counts once, and sets that aren't saved don't count
	if len(creators) != 2 || creators[0] != (CreatorSetCount{Username: "user", Sets: 2}) || creators[1] != (CreatorSetCount{Username: "other", Sets: 1}) {
		t.Errorf("creators = %+v", creators)
	}
}

func TestGetAllSavedSetsSortsByDateDownloaded(t *testing.T) {
	db := openTestMediaDB(t)
	seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 1)
	seedMediaItem(t, db.conn, "2", "Movies", "", "101", "movie", 2)
	// The older date is saved in UTC, so it sorts after the newer one as text
	exec(t, db.conn, `UPDATE SavedItems SET last_downloaded = '2026-10-18 03:14:38.253+00:00' WHERE tmdb_id = '1';`)
	exec(t, db.conn, `UPDATE SavedItems SET last_downloaded = '2026-10-18 01:14:33.61098-04:00' WHERE tmdb_id = '2';`)

	out, Err := db.GetAllSavedSets(testContext(), models.DBFilter{SortOption: "date_downloaded", SortOrder: "desc", ItemsPerPage: -1})
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if len(out.Items) != 2 || out.Items[0].MediaItem.TMDB_ID != "2" || out.Items[1].MediaItem.TMDB_ID != "1" {
		t.Errorf("newest first should be 2 then 1, got %+v", out.Items)
	}
}
