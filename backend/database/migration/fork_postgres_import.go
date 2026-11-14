package migration

import (
	"aura/config"
	"aura/database"
	"aura/models"
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
)

// ImportSQLiteOffline is CLI-only: migrations use process-global paths and Client, never call from a running server.
func ImportSQLiteOffline(ctx context.Context, sourcePath string, target *database.PostgresDB) (database.SQLiteImportReport, error) {
	var report database.SQLiteImportReport
	absolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return report, errors.New("invalid SQLite source path")
	}
	stat, err := os.Stat(absolute)
	if err != nil || !stat.Mode().IsRegular() {
		return report, errors.New("SQLite source must be an existing regular file")
	}
	directory, err := os.MkdirTemp("", "aura-sqlite-import-")
	if err != nil {
		return report, errors.New("cannot create private snapshot directory")
	}
	defer os.RemoveAll(directory)
	snapshot := filepath.Join(directory, "AURA.db")
	sourceURL := url.URL{Scheme: "file", Path: absolute}
	source, err := sql.Open("sqlite3", sourceURL.String()+"?mode=ro&_busy_timeout=30000")
	if err != nil {
		return report, errors.New("cannot open SQLite source read-only")
	}
	// VACUUM INTO sees committed WAL pages; copying the main file alone can silently lose them.
	_, err = source.ExecContext(ctx, `VACUUM INTO ?`, snapshot)
	source.Close()
	if err != nil {
		return report, errors.New("cannot create WAL-consistent SQLite snapshot")
	}
	if err = os.Chmod(snapshot, 0600); err != nil {
		return report, errors.New("cannot secure SQLite snapshot")
	}
	oldPath, oldClient := config.ConfigPath, database.Client
	oldDatabasePath, hadDatabasePath := os.LookupEnv(config.DatabasePathEnv)
	config.ConfigPath = directory
	if err = os.Setenv(config.DatabasePathEnv, "AURA.db"); err != nil {
		config.ConfigPath = oldPath
		return report, err
	}
	defer func() {
		config.ConfigPath = oldPath
		database.Client = oldClient
		if hadDatabasePath {
			_ = os.Setenv(config.DatabasePathEnv, oldDatabasePath)
		} else {
			_ = os.Unsetenv(config.DatabasePathEnv)
		}
	}()
	sqlite := &database.SQliteDB{}
	database.Client = sqlite
	defer sqlite.Close(ctx)
	if _, info := sqlite.Init(ctx); info.Message != "" {
		return report, errors.New("cannot initialize SQLite snapshot")
	}
	if _, info := RunMigrations(); info.Message != "" {
		return report, errors.New("cannot upgrade SQLite snapshot: " + info.Message)
	}
	if _, info := sqlite.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1}); info.Message != "" {
		return report, errors.New("SQLite snapshot failed saved-item read validation")
	}
	conn, _, info := sqlite.GetDBConnection(ctx)
	if info.Message != "" {
		return report, errors.New("cannot open upgraded snapshot")
	}
	if _, info = target.Init(ctx); info.Message != "" {
		return report, errors.New(info.Message)
	}
	return target.ImportSQLite(ctx, conn)
}
