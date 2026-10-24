package database

import (
	"aura/models"
	"testing"
)

func TestLibraryDBStates(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 7, 8)
	seedMediaItem(t, db.conn, "2", "Movies", "4K", "101", "movie", 9)
	seedMediaItem(t, db.conn, "3", "Series", "", "102", "show", 10)
	exec(t, db.conn, `INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode) VALUES ('2', 'Movies', '4K', 'always');`)
	exec(t, db.conn, `UPDATE MediaItems SET on_server = 0;`)

	states, Err := db.GetLibraryDBStates(ctx, "Movies")
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if len(states) != 2 || len(states[dbStateKey("1", "")].Sets) != 2 {
		t.Errorf("states = %+v, want two items in Movies, the first with two sets", states)
	}

	saved := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies"}
	ApplyDBState(states, &saved)
	ignored := models.MediaItem{TMDB_ID: "2", LibraryTitle: "Movies", Edition: "4K"}
	ApplyDBState(states, &ignored)
	nothing := models.MediaItem{TMDB_ID: "4", LibraryTitle: "Movies"}
	ApplyDBState(states, &nothing)
	if len(saved.DBSavedSets) != 2 || saved.IgnoredInDB {
		t.Errorf("saved item = %d sets, ignored %v", len(saved.DBSavedSets), saved.IgnoredInDB)
	}
	if !ignored.IgnoredInDB || ignored.IgnoredMode != "always" || len(ignored.DBSavedSets) != 0 {
		t.Errorf("ignored item = ignored %v, mode %q, %d sets", ignored.IgnoredInDB, ignored.IgnoredMode, len(ignored.DBSavedSets))
	}
	if nothing.DBSavedSets == nil || len(nothing.DBSavedSets) != 0 || nothing.IgnoredInDB {
		t.Errorf("item that isn't saved = %+v", nothing)
	}

}

func TestSaveLibraryItemsAndFinishLibraryLoad(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	// A saved movie and an ignored movie that were seen in an earlier load
	seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 7)
	exec(t, db.conn, `INSERT INTO MediaItems (tmdb_id, library_title, rating_key, type, title, year, on_server, last_seen_at) VALUES ('2', 'Movies', '101', 'movie', 'Ignored', 2020, 1, 100);`)
	exec(t, db.conn, `INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode) VALUES ('2', 'Movies', '', 'always');`)
	exec(t, db.conn, `UPDATE MediaItems SET last_seen_at = 100;`)

	// The first load sees the saved movie and a new movie
	first := []models.MediaItem{
		{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "100", Type: "movie", Title: "Saved", Year: 2020},
		{TMDB_ID: "3", LibraryTitle: "Movies", RatingKey: "102", Type: "movie", Title: "New", Year: 2021, AddedAt: 50,
			Guids: []models.MediaItemGuid{{Provider: "tmdb", ID: "3"}}},
		{TMDB_ID: "4", LibraryTitle: "Old Library", RatingKey: "103", Type: "movie", Title: "Elsewhere", Year: 2021},
		{TMDB_ID: "5", LibraryTitle: "Music", RatingKey: "104", Type: "artist", Title: "Not a movie or show"},
	}
	if Err := db.SaveLibraryItems(ctx, first, 200); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems;`); n != 4 {
		t.Errorf("rows = %d, want 4 (only movies and shows are saved)", n)
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE tmdb_id = '3' AND first_seen_at = 200 AND added_at = 50 AND guids LIKE '%tmdb%';`); n != 1 {
		t.Error("the new movie should be saved with when it was first seen")
	}

	// Movies loaded in full from 150, Old Library is no longer used
	if Err := db.FinishLibraryLoad(ctx, []string{"Movies", "Series"}, map[string]int64{"Movies": 150}); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	onServer := func(tmdbID string) int {
		return count(t, db.conn, `SELECT COALESCE(MAX(on_server), -1) FROM MediaItems WHERE tmdb_id = ?;`, tmdbID)
	}
	if onServer("1") != 1 || onServer("3") != 1 {
		t.Error("movies seen in the load should still be on the server")
	}
	if onServer("2") != 0 {
		t.Error("the ignored movie that wasn't seen should be kept, but not on the server")
	}
	if onServer("4") != -1 {
		t.Error("a library item in a library that is no longer used should be removed")
	}

	// A later load sees the new movie again, which keeps when it was first seen
	if Err := db.SaveLibraryItems(ctx, first[1:2], 300); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if n := count(t, db.conn, `SELECT first_seen_at FROM MediaItems WHERE tmdb_id = '3';`); n != 200 {
		t.Errorf("first seen = %d, want 200", n)
	}

	// Series is used but didn't load, so its items are left alone
	exec(t, db.conn, `INSERT INTO MediaItems (tmdb_id, library_title, rating_key, type, title, year, on_server, last_seen_at) VALUES ('6', 'Series', '105', 'show', 'Show', 2020, 1, 100);`)
	if Err := db.FinishLibraryLoad(ctx, []string{"Movies", "Series"}, map[string]int64{"Movies": 250}); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if onServer("6") != 1 {
		t.Error("an item in a library that didn't load should be left alone")
	}
	if onServer("1") != 0 || onServer("3") != 1 {
		t.Error("the saved movie wasn't seen since 250, so it should be kept but not on the server")
	}
}
