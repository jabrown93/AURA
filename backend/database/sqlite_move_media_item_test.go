package database

import (
	"aura/models"
	"context"
	"database/sql"
	"testing"
)

// openTestMediaDB opens a test database with the media item tables
func openTestMediaDB(t *testing.T) *SQliteDB {
	t.Helper()
	db := &SQliteDB{conn: openTestDB(t)}
	if Err := db.CreateTables(testContext()); Err.Message != "" {
		t.Fatalf("CreateTables: %s", Err.Message)
	}
	return db
}

func exec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func count(t *testing.T, conn *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func seedMediaItem(t *testing.T, conn *sql.DB, tmdbID, library, edition, ratingKey, itemType string, posterSetIDs ...int) int64 {
	t.Helper()
	res, err := conn.Exec(`INSERT INTO MediaItems (tmdb_id, library_title, edition, rating_key, type, title, year, on_server) VALUES (?, ?, ?, ?, ?, 'Title', 2024, 1);`,
		tmdbID, library, edition, ratingKey, itemType)
	if err != nil {
		t.Fatal(err)
	}
	rowID, _ := res.LastInsertId()
	for _, posterSetID := range posterSetIDs {
		exec(t, conn, `INSERT OR IGNORE INTO PosterSets (id, set_id, type, title, user) VALUES (?, ?, 'movie', 'Set', 'user');`, posterSetID, posterSetID)
		exec(t, conn, `INSERT INTO SavedItems (tmdb_id, library_title, edition, poster_set_id, poster_selected, last_downloaded) VALUES (?, ?, ?, ?, 1, '2026-10-01');`,
			tmdbID, library, edition, posterSetID)
	}
	return rowID
}

func TestMoveMediaItem(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	rowID := seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 1)
	exec(t, db.conn, `INSERT INTO Movies (media_item_id, path, size, duration) VALUES (?, '/old/movie.mkv', 1, 1);`, rowID)
	exec(t, db.conn, `INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode) VALUES ('1', 'Movies', '', 'always');`)
	exec(t, db.conn, `INSERT INTO DownloadHistory (tmdb_id, library_title, edition, media_item_title, media_item_year, set_id, set_title, status) VALUES ('1', 'Movies', '', 'Title', 2024, '1', 'Set', 'success');`)

	newItem := models.MediaItem{LibraryTitle: "Films", RatingKey: "movie-tmdb-1", Type: "movie", Title: "Title", Year: 2024,
		Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/new/movie.mkv", Size: 2, Duration: 2}}}
	if Err := db.MoveMediaItem(ctx, "1", "Movies", "", newItem, true); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE tmdb_id = '1' AND library_title = 'Films' AND rating_key = 'movie-tmdb-1' AND on_server = 1;`); n != 1 {
		t.Errorf("moved media items = %d, want 1", n)
	}
	for _, table := range []string{"SavedItems", "IgnoredItems", "DownloadHistory"} {
		if n := count(t, db.conn, `SELECT COUNT(*) FROM `+table+` WHERE tmdb_id = '1' AND library_title = 'Films';`); n != 1 {
			t.Errorf("%s rows moved = %d, want 1", table, n)
		}
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Movies WHERE path = '/new/movie.mkv';`); n != 1 {
		t.Error("the movie's file should come from the new main media server")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems;`); n != 1 {
		t.Errorf("media items = %d, want 1", n)
	}
}

func TestMoveMediaItemMergesEditions(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	// The new main media server has no editions, so Directors Cut becomes the base movie, which is also saved
	cutRowID := seedMediaItem(t, db.conn, "2", "4K Movies", "Directors Cut", "200", "movie", 10, 11)
	exec(t, db.conn, `INSERT INTO Movies (media_item_id, path, size, duration) VALUES (?, '/cut.mkv', 1, 1);`, cutRowID)
	seedMediaItem(t, db.conn, "2", "4K Movies", "", "201", "movie", 11, 12)

	newItem := models.MediaItem{LibraryTitle: "4K Movies", Edition: "", RatingKey: "movie-tmdb-2", Type: "movie", Title: "Title", Year: 2024,
		Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/silo.mkv"}}}
	if Err := db.MoveMediaItem(ctx, "2", "4K Movies", "Directors Cut", newItem, true); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE tmdb_id = '2';`); n != 1 {
		t.Errorf("media items = %d, want 1 after merging", n)
	}
	// Sets 10, 11 and 12 are all saved for the merged item, set 11 once
	if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE tmdb_id = '2' AND library_title = '4K Movies' AND edition = '';`); n != 3 {
		t.Errorf("saved sets = %d, want 3", n)
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE edition = 'Directors Cut';`); n != 0 {
		t.Error("no saved sets should be left on the old edition")
	}
	// The merged item's old movie row is removed
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Movies WHERE media_item_id = ?;`, cutRowID); n != 0 {
		t.Error("the old edition's movie row should be removed")
	}
}

func TestMoveMediaItemNotFound(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	seedMediaItem(t, db.conn, "3", "Kids", "", "300", "movie", 20)
	seedMediaItem(t, db.conn, "4", "Movies", "", "400", "movie", 21)

	// No matching library on the new main media server: only marked as not on the server
	if Err := db.MoveMediaItem(ctx, "3", "Kids", "", models.MediaItem{}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	// A matching library, but the item isn't in it
	if Err := db.MoveMediaItem(ctx, "4", "Movies", "", models.MediaItem{LibraryTitle: "Films"}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE tmdb_id = '3' AND library_title = 'Kids' AND rating_key = '300' AND on_server = 0;`); n != 1 {
		t.Error("the item should be kept and marked as not on the server")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE tmdb_id = '4' AND library_title = 'Films' AND on_server = 0;`); n != 1 {
		t.Error("the item should move to the matching library and be marked as not on the server")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE tmdb_id = '4' AND library_title = 'Films';`); n != 1 {
		t.Error("the saved set should move with it")
	}
}

func TestMoveMediaItemShow(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	rowID := seedMediaItem(t, db.conn, "5", "Series", "", "500", "show", 30)
	res, _ := db.conn.Exec(`INSERT INTO Series (media_item_id, season_count, episode_count, location) VALUES (?, 1, 2, '/old/show');`, rowID)
	seriesID, _ := res.LastInsertId()
	res, _ = db.conn.Exec(`INSERT INTO Seasons (series_id, rating_key, season_number, episode_count) VALUES (?, 'old-season', 1, 2);`, seriesID)
	seasonID, _ := res.LastInsertId()
	exec(t, db.conn, `INSERT INTO Episodes (season_id, rating_key, episode_number, title, path, size, duration) VALUES (?, 'old-ep1', 1, 'One', '/old/1.mkv', 1, 1), (?, 'old-ep2', 2, 'Two', '/old/2.mkv', 1, 1);`, seasonID, seasonID)

	// The new main media server is missing episode 2
	newItem := models.MediaItem{LibraryTitle: "Shows", RatingKey: "series-tvdb-5", Type: "show", Title: "Title", Year: 2024,
		Series: &models.MediaItemSeries{SeasonCount: 1, EpisodeCount: 1, Location: "/new/show", Seasons: []models.MediaItemSeason{
			{RatingKey: "season-tvdb-5-1", SeasonNumber: 1, Episodes: []models.MediaItemEpisode{
				{RatingKey: "episode-1", SeasonNumber: 1, EpisodeNumber: 1, Title: "One", File: models.MediaItemFile{Path: "/new/1.mkv"}},
			}},
		}}}
	if Err := db.MoveMediaItem(ctx, "5", "Series", "", newItem, true); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	if n := count(t, db.conn, `SELECT COUNT(*) FROM Seasons WHERE rating_key = 'season-tvdb-5-1';`); n != 1 {
		t.Error("the season should get the new main media server's rating key")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Episodes;`); n != 1 {
		t.Errorf("episodes = %d, want 1", n)
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Episodes WHERE rating_key = 'episode-1' AND path = '/new/1.mkv';`); n != 1 {
		t.Error("the episode should come from the new main media server")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE library_title = 'Shows' AND rating_key = 'series-tvdb-5';`); n != 1 {
		t.Error("the show should move to the new library")
	}
}

func TestCountActiveDownloadQueueJobs(t *testing.T) {
	db := openTestMediaDB(t)
	exec(t, db.conn, `INSERT INTO DownloadQueueJobs (tmdb_id, library_title, media_item_title, payload, status) VALUES
		('1', 'Movies', 'A', '{}', 'pending'), ('2', 'Movies', 'B', '{}', 'processing'), ('3', 'Movies', 'C', '{}', 'success');`)
	if n, Err := db.CountActiveDownloadQueueJobs(testContext()); Err.Message != "" || n != 2 {
		t.Errorf("active jobs = %d (%s), want 2", n, Err.Message)
	}
}

func TestMoveMediaItemsIsAllOrNothing(t *testing.T) {
	db := openTestMediaDB(t)
	seedMediaItem(t, db.conn, "6", "Movies", "", "600", "movie", 40)

	// A cancelled context makes the move fail
	moves := []MediaItemMove{
		{TMDB_ID: "6", OldLibrary: "Movies", NewItem: models.MediaItem{LibraryTitle: "Films", RatingKey: "new", Type: "movie", Title: "Title", Year: 2024}, Found: true},
	}
	ctx, cancel := context.WithCancel(testContext())
	cancel()
	if Err := db.MoveMediaItems(ctx, moves); Err.Message == "" {
		t.Fatal("expected an error with a cancelled context")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE library_title = 'Movies' AND rating_key = '600';`); n != 1 {
		t.Error("nothing should change when the move fails")
	}
}
