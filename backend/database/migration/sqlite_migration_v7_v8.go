package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_7_to_8 adds the SiloItemIDs table, which saves the provider IDs of Silo
// catalog items so they do not need to be retrieved from Silo after each restart.
func migrate_7_to_8(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v7 to v8", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 7).Int("To Version", 8).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	backupErr := database.Backup(ctx, 7, 8)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS SiloItemIDs (
    content_id TEXT PRIMARY KEY,
    tmdb_id TEXT NOT NULL DEFAULT '',
    imdb_id TEXT NOT NULL DEFAULT '',
    tvdb_id TEXT NOT NULL DEFAULT '',
    first_air_date TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
	`); err != nil {
		logAction.SetError("Failed to create SiloItemIDs table", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v7.0 to v8.0 completed successfully")
	return Err
}
