package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_12_to_13 adds the Auto Upgrade setting of each saved set, every set can be upgraded to start
func migrate_12_to_13(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v12 to v13", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 12).Int("To Version", 13).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 12, 13)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	exists, checkErr := checkColumnExists(ctx, "SavedItems", "no_auto_upgrade")
	if checkErr.Message != "" {
		return checkErr
	}
	if !exists {
		if _, err := conn.ExecContext(ctx, `ALTER TABLE SavedItems ADD COLUMN no_auto_upgrade INTEGER NOT NULL DEFAULT 0 CHECK (no_auto_upgrade IN (0,1));`); err != nil {
			logAction.SetError("Failed to add the no_auto_upgrade column to the SavedItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v12.0 to v13.0 completed successfully")
	return logging.LogErrorInfo{}
}
