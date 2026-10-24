package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// downloadHistoryColumns are the DownloadHistory columns for the set's creator, selected types and warnings
var downloadHistoryColumns = []struct{ name, definition string }{
	{"set_creator", "TEXT NOT NULL DEFAULT ''"},
	{"selected_types", "TEXT NOT NULL DEFAULT '{}'"},
	{"warning_count", "INTEGER NOT NULL DEFAULT 0"},
}

// migrate_9_to_10 adds the set's details to DownloadHistory and removes the successful downloads, it only keeps failures now
func migrate_9_to_10(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v9 to v10", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 9).Int("To Version", 10).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 9, 10)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	for _, column := range downloadHistoryColumns {
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

	result, err := conn.ExecContext(ctx, "DELETE FROM DownloadHistory WHERE status = 'success';")
	if err != nil {
		logAction.SetError("Failed to remove the successful downloads from DownloadHistory", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	if deleted, err := result.RowsAffected(); err == nil {
		logAction.AppendResult("successful_downloads_removed", deleted)
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v9.0 to v10.0 completed successfully")
	return logging.LogErrorInfo{}
}
