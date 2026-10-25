package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabasePath(t *testing.T) {
	ConfigPath = t.TempDir()
	t.Setenv(DatabasePathEnv, "")

	if got, want := DatabasePath(), filepath.Join(ConfigPath, "AURA.db"); got != want {
		t.Errorf("default DatabasePath() = %q, want %q", got, want)
	}

	// A custom Database.Path from an older config file is still used
	legacyConfig := []byte("Database:\n  Type: sqlite3\n  Path: data/custom.db\n")
	if err := os.WriteFile(filepath.Join(ConfigPath, "config.yaml.imported"), legacyConfig, 0o600); err != nil {
		t.Fatal(err)
	}
	databasePathCache.key = ""
	if got, want := DatabasePath(), filepath.Join(ConfigPath, "data/custom.db"); got != want {
		t.Errorf("DatabasePath() from config file = %q, want %q", got, want)
	}

	// DATABASE_PATH takes priority, relative to the config path
	t.Setenv(DatabasePathEnv, "other.db")
	if got, want := DatabasePath(), filepath.Join(ConfigPath, "other.db"); got != want {
		t.Errorf("DatabasePath() from relative %s = %q, want %q", DatabasePathEnv, got, want)
	}

	// or an absolute path
	absolutePath := filepath.Join(t.TempDir(), "absolute.db")
	t.Setenv(DatabasePathEnv, absolutePath)
	if got := DatabasePath(); got != absolutePath {
		t.Errorf("DatabasePath() from absolute %s = %q, want %q", DatabasePathEnv, got, absolutePath)
	}
}
