package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"sync/atomic"
	"testing"

	"github.com/mattn/go-sqlite3"
)

func TestGetAllMediaItemStatesReturnsSavedSetsAndIgnoreModesInOneBulkRead(t *testing.T) {
	var selectStatements, updateStatements atomic.Int32
	sql.Register("sqlite3_media_state_counts", &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
		conn.RegisterAuthorizer(func(operation int, _, _, _ string) int {
			switch operation {
			case sqlite3.SQLITE_SELECT:
				selectStatements.Add(1)
			case sqlite3.SQLITE_UPDATE:
				updateStatements.Add(1)
			}
			return sqlite3.SQLITE_OK
		})
		return nil
	}})
	conn, err := sql.Open("sqlite3_media_state_counts", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	statements := []string{
		`CREATE TABLE MediaItems (tmdb_id TEXT, library_title TEXT, on_server INTEGER DEFAULT 0)`,
		`CREATE TABLE IgnoredItems (tmdb_id TEXT, library_title TEXT, mode TEXT, current_sets TEXT)`,
		`CREATE TABLE PosterSets (id INTEGER PRIMARY KEY, set_id TEXT, user TEXT)`,
		`CREATE TABLE SavedItems (
			tmdb_id TEXT, library_title TEXT, poster_set_id INTEGER,
			poster_selected INTEGER, backdrop_selected INTEGER,
			season_poster_selected INTEGER, special_season_poster_selected INTEGER,
			titlecard_selected INTEGER
		)`,
		`INSERT INTO MediaItems(tmdb_id, library_title) VALUES ('1', 'Movies'), ('2', 'Movies'), ('3', 'Movies')`,
		`INSERT INTO IgnoredItems VALUES
			('2', 'Movies', ' until-set-available ', 'set-a,set-b'),
			('4', 'Movies', 'always', '')`,
		`INSERT INTO PosterSets VALUES (7, 'set-7', 'creator')`,
		`INSERT INTO SavedItems VALUES
			('1', 'Movies', 7, 1, 0, 1, 0, 1),
			('1', 'Movies', 7, 1, 0, 1, 0, 1),
			('2', 'Movies', 7, 1, 0, 1, 0, 1),
			('3', 'Movies', 99, 1, 0, 0, 0, 0)`,
	}
	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	db := &SQliteDB{conn: conn}
	ctx, logData := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, logData.AddAction("bulk states", logging.LevelDebug))
	selectStatements.Store(0)
	states, logErr := db.GetAllMediaItemStates(ctx)
	if logErr.Message != "" {
		t.Fatalf("GetAllMediaItemStates() error = %s", logErr.Message)
	}
	if got := selectStatements.Load(); got != 1 {
		t.Fatalf("GetAllMediaItemStates() prepared %d SELECT statements, want 1", got)
	}

	saved := states[MediaItemKey{TMDBID: "1", LibraryTitle: "Movies"}]
	if saved.Ignored || len(saved.SavedSets) != 1 || saved.SavedSets[0].ID != "set-7" {
		t.Fatalf("saved state = %+v", saved)
	}
	if !saved.SavedSets[0].SelectedTypes.Poster || !saved.SavedSets[0].SelectedTypes.SeasonPoster || !saved.SavedSets[0].SelectedTypes.Titlecard {
		t.Fatalf("saved selected types = %+v", saved.SavedSets[0].SelectedTypes)
	}

	ignored := states[MediaItemKey{TMDBID: "2", LibraryTitle: "Movies"}]
	if !ignored.Ignored || ignored.IgnoreMode != "until-set-available" || len(ignored.IgnoredSets) != 2 || ignored.IgnoredSets[0] != "set-a" || ignored.IgnoredSets[1] != "set-b" || len(ignored.SavedSets) != 1 || ignored.SavedSets[0].ID != "set-7" {
		t.Fatalf("ignored state = %+v, want ignore and saved-set state retained independently", ignored)
	}

	unresolved := states[MediaItemKey{TMDBID: "3", LibraryTitle: "Movies"}]
	if unresolved.Ignored || len(unresolved.SavedSets) != 0 {
		t.Fatalf("unresolved poster-set state = %+v, want no saved sets", unresolved)
	}

	ignoredWithoutMediaItem := states[MediaItemKey{TMDBID: "4", LibraryTitle: "Movies"}]
	if !ignoredWithoutMediaItem.Ignored || ignoredWithoutMediaItem.IgnoreMode != "always" {
		t.Fatalf("ignored-only state = %+v, want always ignored", ignoredWithoutMediaItem)
	}

	updateStatements.Store(0)
	if logErr := db.UpdateMediaItemsOnServer(ctx, "Movies", []string{"1", "2"}, true); logErr.Message != "" {
		t.Fatalf("UpdateMediaItemsOnServer() error = %s", logErr.Message)
	}
	var updated int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM MediaItems WHERE on_server = 1`).Scan(&updated); err != nil {
		t.Fatal(err)
	}
	if updated != 2 {
		t.Fatalf("bulk-updated rows = %d, want 2", updated)
	}
	if got := updateStatements.Load(); got != 1 {
		t.Fatalf("UpdateMediaItemsOnServer() prepared %d UPDATE statements, want 1", got)
	}
}
