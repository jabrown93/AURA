package migration

import (
	"aura/config"
	"aura/database"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestOfflineImportUpgradesOnlySnapshot proves old fork state survives while source bytes and WAL state stay untouched.
func TestOfflineImportUpgradesOnlySnapshot(t *testing.T) {
	dsn := os.Getenv("AURA_TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("AURA_TEST_POSTGRES_DSN is required in CI")
		}
		t.Skip("AURA_TEST_POSTGRES_DSN is not set")
	}
	ctx := context.Background()
	control, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	random := make([]byte, 8)
	if _, err = rand.Read(random); err != nil {
		t.Fatal(err)
	}
	schema := "aura_import_test_" + hex.EncodeToString(random)
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err = control.Exec("CREATE SCHEMA " + identifier); err != nil {
		t.Fatal(err)
	}
	defer control.Exec("DROP SCHEMA " + identifier + " CASCADE")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	t.Setenv("AURA_DB_DRIVER", "postgres")
	t.Setenv("AURA_POSTGRES_DSN", parsed.String())
	client, info := database.NewDatabaseClient()
	if info.Message != "" {
		t.Fatal(info)
	}
	target := client.(*database.PostgresDB)
	defer target.Close(ctx)
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := sql.Open("sqlite3", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dump, err := os.ReadFile("testdata/fork_v6.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.Exec(string(dump)); err != nil {
		t.Fatal(err)
	}
	// Keep a committed change in WAL, which a raw source-file copy would miss.
	if _, err = source.Exec(`PRAGMA journal_mode=WAL; UPDATE MediaItems SET title='From WAL' WHERE tmdb_id='1399'`); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	oldPath, oldClient := config.ConfigPath, database.Client
	if _, err = ImportSQLiteOffline(ctx, sourcePath, target); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("source database bytes changed")
	}
	if config.ConfigPath != oldPath || database.Client != oldClient {
		t.Fatal("offline globals were not restored")
	}
	conn, _, info := target.GetDBConnection(ctx)
	if info.Message != "" {
		t.Fatal(info)
	}
	var title string
	var preload int
	if err = conn.QueryRow(`SELECT m.title,s.force_preload_missing FROM MediaItems m JOIN SavedItems s USING(tmdb_id,library_title,edition) WHERE m.tmdb_id='1399'`).Scan(&title, &preload); err != nil || title != "From WAL" || preload != 1 {
		t.Fatalf("import: %q %d %v", title, preload, err)
	}
	var version int
	if err = source.QueryRow(`SELECT version FROM VERSION`).Scan(&version); err != nil || version != 6 {
		t.Fatalf("source version changed: %d %v", version, err)
	}
}
