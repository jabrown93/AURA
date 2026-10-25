package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupIncludesWritesStillInWAL(t *testing.T) {
	conn := openTestDB(t)
	if _, err := conn.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("INSERT INTO items (id) VALUES (1), (2), (3)"); err != nil {
		t.Fatal(err)
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	if Err := (&SQliteDB{conn: conn}).Backup(ctx, 1, 2); Err.Message != "" {
		t.Fatalf("Backup: %s", Err.Message)
	}

	matches, err := filepath.Glob(filepath.Join(config.ConfigPath, "test_backup_v1_to_v2_*.db"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one backup file, got %v (err %v)", matches, err)
	}
	backup, err := sql.Open("sqlite3", matches[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()

	var count int
	if err := backup.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if count != 3 {
		t.Errorf("backup has %d rows, want 3", count)
	}
}

func TestCreateBackupCopiesTheDatabase(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}
	if _, err := db.conn.Exec("CREATE TABLE items (n INTEGER NOT NULL); INSERT INTO items (n) VALUES (1), (2);"); err != nil {
		t.Fatal(err)
	}

	backup, Err := db.CreateBackup(testContext(), 7)
	if Err.Message != "" {
		t.Fatalf("CreateBackup: %s", Err.Message)
	}
	if backup.Size == 0 {
		t.Error("the backup should not be empty")
	}

	conn, err := sql.Open("sqlite3", filepath.Join(BackupsFolder(), backup.Name))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM items").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("items in the backup = %d, want 2", n)
	}
}

func TestCreateBackupKeepsTheNewest(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}

	// Files that aren't backups aura made are never deleted
	folder := BackupsFolder()
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	others := []string{"notes.txt", "test_backup_v7_to_v8_20260101_000000.db", "other_2026-01-01_00-00-00.db"}
	for _, name := range others {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var made []string
	for range 4 {
		backup, Err := db.CreateBackup(testContext(), 2)
		if Err.Message != "" {
			t.Fatalf("CreateBackup: %s", Err.Message)
		}
		made = append(made, backup.Name)
	}

	backups, err := ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 || backups[0].Name != made[3] || backups[1].Name != made[2] {
		var names []string
		for _, b := range backups {
			names = append(names, b.Name)
		}
		t.Errorf("backups = %v, want the two newest %v", names, []string{made[3], made[2]})
	}
	for _, name := range others {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			t.Errorf("%s was deleted", name)
		}
	}
}

func TestListBackupsNewestFirst(t *testing.T) {
	openTestDB(t)
	folder := BackupsFolder()
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test_2026-01-01_00-00-00.db", "test_2026-01-01_00-00-00_10.db", "test_2026-01-01_00-00-00_2.db", "test_2026-02-01_00-00-00.db"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	backups, err := ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"test_2026-02-01_00-00-00.db", "test_2026-01-01_00-00-00_10.db", "test_2026-01-01_00-00-00_2.db", "test_2026-01-01_00-00-00.db"}
	if len(backups) != len(want) {
		t.Fatalf("got %d backups, want %d", len(backups), len(want))
	}
	for i, backup := range backups {
		if backup.Name != want[i] {
			t.Errorf("backup %d = %s, want %s", i, backup.Name, want[i])
		}
	}
}
