package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	config.ConfigPath = dir
	t.Setenv(config.DatabasePathEnv, "test.db")
	// Pre-create the file so GetDBConnection skips the new-database log path.
	if err := os.WriteFile(filepath.Join(dir, "test.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	conn, _, Err := (&SQliteDB{}).GetDBConnection(ctx)
	if Err.Message != "" {
		t.Fatalf("GetDBConnection: %s", Err.Message)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestGetDBConnectionUsesWALAndBusyTimeout(t *testing.T) {
	conn := openTestDB(t)

	var journalMode string
	if err := conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var busyTimeout int
	if err := conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != sqliteBusyTimeoutMs {
		t.Errorf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMs)
	}
}

// Transactions that read before writing fail with SQLITE_BUSY under deferred
// locking, even with a busy timeout. This mirrors the queue worker and the
// Plex listener writing at the same time.
func TestConcurrentReadThenWriteTransactionsDoNotFail(t *testing.T) {
	conn := openTestDB(t)
	if _, err := conn.Exec("CREATE TABLE counter (n INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO counter (n) VALUES (0)"); err != nil {
		t.Fatal(err)
	}

	const workers, iterations = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, workers*iterations)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				tx, err := conn.Begin()
				if err != nil {
					errs <- fmt.Errorf("begin: %w", err)
					continue
				}
				var n int
				if err := tx.QueryRow("SELECT n FROM counter").Scan(&n); err != nil {
					tx.Rollback()
					errs <- fmt.Errorf("read: %w", err)
					continue
				}
				if _, err := tx.Exec("UPDATE counter SET n = ?", n+1); err != nil {
					tx.Rollback()
					errs <- fmt.Errorf("write: %w", err)
					continue
				}
				if err := tx.Commit(); err != nil {
					errs <- fmt.Errorf("commit: %w", err)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)

	failures := 0
	for err := range errs {
		if failures < 3 {
			t.Error(err)
		}
		failures++
	}
	if failures > 0 {
		t.Fatalf("%d of %d transactions failed", failures, workers*iterations)
	}

	var n int
	if err := conn.QueryRow("SELECT n FROM counter").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != workers*iterations {
		t.Errorf("counter = %d, want %d", n, workers*iterations)
	}
}

func TestCloseRemovesWALFiles(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}
	if _, err := db.conn.Exec("CREATE TABLE items (n INTEGER NOT NULL); INSERT INTO items (n) VALUES (1);"); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(config.ConfigPath, "test.db")
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Fatalf("expected a WAL file before closing: %v", err)
	}

	if Err := db.Close(testContext()); Err.Message != "" {
		t.Fatalf("Close: %s", Err.Message)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
			t.Errorf("test.db%s still exists after Close", suffix)
		}
	}

	// The data is in the database file
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM items").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("items = %d, want 1", n)
	}
}

func TestGetDBConnectionReusesTheOpenConnection(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}
	conn, _, Err := db.GetDBConnection(testContext())
	if Err.Message != "" {
		t.Fatalf("GetDBConnection: %s", Err.Message)
	}
	if conn != db.conn {
		t.Error("GetDBConnection opened a new connection instead of using the open one")
	}
}

func TestCheckFailsOnceTheDatabaseIsClosed(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}
	if err := db.Check(testContext()); err != nil {
		t.Fatalf("Check on an open database: %v", err)
	}
	if Err := db.Close(testContext()); Err.Message != "" {
		t.Fatalf("Close: %s", Err.Message)
	}
	if err := db.Check(testContext()); err == nil {
		t.Error("Check should fail once the database is closed")
	}
	if err := (&SQliteDB{}).Check(testContext()); err == nil {
		t.Error("Check should fail when the database was never opened")
	}
}
