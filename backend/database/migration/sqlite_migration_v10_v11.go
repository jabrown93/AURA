package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// downloadRetryColumns are the DownloadHistory columns for retrying failed downloads
var downloadRetryColumns = []struct{ name, definition string }{
	{"retry_count", "INTEGER NOT NULL DEFAULT 0"},
	{"next_retry_at", "DATETIME"},
}

// migrate_10_to_11 adds how many times a failed download was retried, and when it's retried next
func migrate_10_to_11(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v10 to v11", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 10).Int("To Version", 11).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 10, 11)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	for _, column := range downloadRetryColumns {
		exists, checkErr := checkColumnExists(ctx, "DownloadHistory", column.name)
		if checkErr.Message != "" {
			return checkErr
		}
		if exists {
			continue
		}
		if _, err := conn.ExecContext(ctx, "ALTER TABLE DownloadHistory ADD COLUMN "+column.name+" "+column.definition+";"); err != nil {
			logAction.SetError("Failed to add a column to the DownloadHistory table", "", map[string]any{"column": column.name, "error": err.Error()})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v10.0 to v11.0 completed successfully")
	return logging.LogErrorInfo{}
}
