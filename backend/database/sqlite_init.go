package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

// The download queue worker, Plex event listener and API handlers write concurrently.
// WAL stops readers blocking the writer, the busy timeout makes writers wait instead of
// failing, and immediate transactions take the write lock up front: a deferred
// transaction that upgrades from read to write fails at once with SQLITE_BUSY.
const sqliteBusyTimeoutMs = 30000

var sqliteConnectionParams = fmt.Sprintf("?_journal_mode=WAL&_busy_timeout=%d&_txlock=immediate", sqliteBusyTimeoutMs)

func (s *SQliteDB) Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Initializing Database", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	newDB = false

	// Open DB Connection
	s.conn, newDB, Err = s.GetDBConnection(ctx)
	if Err.Message != "" {
		return newDB, Err
	}

	// Create the config tables here, as the config is loaded before the database migrations run
	Err = s.CreateConfigTables(ctx)
	if Err.Message != "" {
		return newDB, Err
	}

	// If new DB, create version table, main tables and set version
	if newDB {
		Err = s.CreateVersionTable(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.CreateAuthTable(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.CreateTables(ctx)
		if Err.Message != "" {
			return newDB, Err
		}

		Err = s.UpdateVersionTable(ctx, LATEST_DB_VERSION)
		if Err.Message != "" {
			return newDB, Err
		}
	}

	return newDB, Err
}

func (s *SQliteDB) GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Opening Database Connection", logging.LevelInfo)
	defer logAction.Complete()

	conn = nil
	newDB = false
	Err = logging.LogErrorInfo{}

	// Use the open connection, so the last one closed on shutdown removes the WAL files
	if s.conn != nil {
		return s.conn, false, Err
	}

	// Get the DB path
	dbPath := config.DatabasePath()

	// Check if DB file exists
	_, statErr := os.Stat(dbPath)
	newDB = os.IsNotExist(statErr)
	if newDB {
		logging.LOGGER.Warn().Timestamp().Msg("Database file not found. Creating new database.")
	}

	conn, openErr := sql.Open("sqlite3", dbPath+sqliteConnectionParams)
	if openErr != nil {
		return conn, newDB, logging.LogErrorInfo{Message: openErr.Error()}
	}

	// Ping to verify connection
	if pingErr := conn.PingContext(ctx); pingErr != nil {
		return conn, newDB, logging.LogErrorInfo{Message: pingErr.Error()}
	}

	return conn, newDB, Err
}

// Close moves the WAL into the database file and closes the connection.
// SQLite removes the -wal and -shm files when the last connection closes.
func (s *SQliteDB) Close(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Closing Database", logging.LevelInfo)
	defer logAction.Complete()

	if s.conn == nil {
		return logging.LogErrorInfo{}
	}
	if _, err := s.conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		logAction.AppendWarning("checkpoint_error", err.Error())
	}
	if err := s.conn.Close(); err != nil {
		logAction.SetError("Failed to close the database", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

// Check runs a query to make sure the database can be read
func (s *SQliteDB) Check(ctx context.Context) error {
	if s.conn == nil {
		return errors.New("the database is not open")
	}
	var one int
	return s.conn.QueryRowContext(ctx, "SELECT 1;").Scan(&one)
}
