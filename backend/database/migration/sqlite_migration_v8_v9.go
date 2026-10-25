package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// mediaItemsLibraryColumns are the MediaItems columns for the whole main library
var mediaItemsLibraryColumns = []struct{ name, definition string }{
	{"added_at", "INTEGER NOT NULL DEFAULT 0"},
	{"updated_at", "INTEGER NOT NULL DEFAULT 0"},
	{"released_at", "INTEGER NOT NULL DEFAULT 0"},
	{"latest_episode_added_at", "INTEGER NOT NULL DEFAULT 0"},
	{"content_rating", "TEXT NOT NULL DEFAULT ''"},
	{"summary", "TEXT NOT NULL DEFAULT ''"},
	{"guids", "TEXT NOT NULL DEFAULT '[]'"},
	{"first_seen_at", "INTEGER NOT NULL DEFAULT 0"},
	{"last_seen_at", "INTEGER NOT NULL DEFAULT 0"},
}

// migrate_8_to_9 adds the columns that let MediaItems hold the whole main library
func migrate_8_to_9(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v8 to v9", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 8).Int("To Version", 9).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 8, 9)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	for _, column := range mediaItemsLibraryColumns {
		exists, checkErr := checkColumnExists(ctx, "MediaItems", column.name)
		if checkErr.Message != "" {
			return checkErr
		}
		if exists {
			continue
		}
		if _, err := conn.ExecContext(ctx, "ALTER TABLE MediaItems ADD COLUMN "+column.name+" "+column.definition+";"); err != nil {
			logAction.SetError("Failed to add a column to the MediaItems table", "", map[string]any{"column": column.name, "error": err.Error()})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v8.0 to v9.0 completed successfully")
	return logging.LogErrorInfo{}
}
