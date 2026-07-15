package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_5_to_6 adds the force_preload_missing column to the SavedItems table.
// This per-set flag pre-stages season-poster/titlecard images for seasons/episodes
// missing from the media server by writing them to the Kometa asset directory.
// A plain ADD COLUMN with a constant default is sufficient here (no CHECK-constraint
// change on existing columns, so no table rebuild is required).
func migrate_5_to_6(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v5 to v6", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 5).Int("To Version", 6).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	// Create a backup of the current database
	backupErr := database.Backup(ctx, 5, 6)
	if backupErr.Message != "" {
		return backupErr
	}

	// Get DB connection
	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	// Check if the "force_preload_missing" column already exists to avoid a duplicate column error
	columnExists, checkColumnErr := checkColumnExists(ctx, "SavedItems", "force_preload_missing")
	if checkColumnErr.Message != "" {
		return checkColumnErr
	}

	if !columnExists {
		addColumnQuery := `ALTER TABLE SavedItems ADD COLUMN force_preload_missing INTEGER NOT NULL DEFAULT 0 CHECK (force_preload_missing IN (0,1));`
		if _, err := conn.ExecContext(ctx, addColumnQuery); err != nil {
			logAction.SetError("Failed to add force_preload_missing column to SavedItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v5.0 to v6.0 completed successfully")
	return logging.LogErrorInfo{}
}
