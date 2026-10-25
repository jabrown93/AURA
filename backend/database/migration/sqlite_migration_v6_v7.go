package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_6_to_7 adds the DownloadQueueJobs and DownloadHistory tables,
// replacing the old JSON-file + cron-polling download queue with a
// SQLite-backed job queue and a persisted download history.
func migrate_6_to_7(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v6 to v7", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 6).Int("To Version", 7).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	backupErr := database.Backup(ctx, 6, 7)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to begin transaction for creating download queue/history tables", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if _, err = tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS DownloadQueueJobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    media_item_title TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','processing','success','warning','error')),
    result_message TEXT NOT NULL DEFAULT '',
    result_errors TEXT NOT NULL DEFAULT '[]',
    result_warnings TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    finished_at DATETIME,
    worker_id TEXT NOT NULL DEFAULT ''
);
	`); err != nil {
		tx.Rollback()
		logAction.SetError("Failed to create DownloadQueueJobs table", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if _, err = tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_downloadqueuejobs_status ON DownloadQueueJobs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_downloadqueuejobs_item ON DownloadQueueJobs(tmdb_id, library_title, edition);
	`); err != nil {
		tx.Rollback()
		logAction.SetError("Failed to create indexes on DownloadQueueJobs table", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if _, err = tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS DownloadHistory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    media_item_title TEXT NOT NULL,
    media_item_year INTEGER NOT NULL,
    set_id TEXT NOT NULL,
    set_title TEXT NOT NULL,
    images_succeeded INTEGER NOT NULL DEFAULT 0,
    images_failed INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('success','warning','error')),
    failed_images TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
	`); err != nil {
		tx.Rollback()
		logAction.SetError("Failed to create DownloadHistory table", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if _, err = tx.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_downloadhistory_status_created ON DownloadHistory(status, created_at);
	`); err != nil {
		tx.Rollback()
		logAction.SetError("Failed to create indexes on DownloadHistory table", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if err = tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction for creating download queue/history tables", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v6.0 to v7.0 completed successfully")
	return Err
}
