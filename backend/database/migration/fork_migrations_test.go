package migration

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// A database the fork migrated to its own v6 (force_preload_missing, no edition column) must run
// upstream's v6..latest and keep its force_preload_missing values.
func TestForkV6DatabaseRunsUpstreamMigrations(t *testing.T) {
	dir := t.TempDir()
	oldPath := config.ConfigPath
	config.ConfigPath = dir
	t.Cleanup(func() { config.ConfigPath = oldPath; database.Client = nil })

	dump, err := os.ReadFile(filepath.Join("testdata", "fork_v6.sql"))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := sql.Open("sqlite3", config.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seed.Exec(string(dump)); err != nil {
		t.Fatalf("loading fixture: %v", err)
	}
	seed.Close()

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelTrace))
	if _, Err := database.Init(ctx); Err.Message != "" {
		t.Fatalf("Init: %s", Err.Message)
	}

	if _, Err := RunMigrations(); Err.Message != "" {
		t.Fatalf("RunMigrations: %s %v", Err.Message, Err.Detail)
	}
	if v, _ := database.GetCurrentVersion(ctx); v != database.LATEST_DB_VERSION {
		t.Fatalf("version = %d, want %d", v, database.LATEST_DB_VERSION)
	}
	if n, Err := RunMigrations(); n != 0 || Err.Message != "" {
		t.Fatalf("second run: %d migrations, err %q", n, Err.Message)
	}

	conn, _, _ := database.GetDBConnection(ctx)
	if ok, _ := checkColumnExists(ctx, "MediaItems", "edition"); !ok {
		t.Error("MediaItems.edition missing: upstream's v5->v6 did not run")
	}
	rows, err := conn.QueryContext(ctx, `SELECT tmdb_id, force_preload_missing FROM SavedItems ORDER BY tmdb_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var id string
		var preload int
		if err := rows.Scan(&id, &preload); err != nil {
			t.Fatal(err)
		}
		got[id] = preload
	}
	if got["1399"] != 1 || got["1396"] != 0 || len(got) != 2 {
		t.Errorf("force_preload_missing = %v, want 1399=1 1396=0", got)
	}
	var stash int
	_ = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, forkPreloadStash).Scan(&stash)
	if stash != 0 {
		t.Error("stash table left behind")
	}
}
