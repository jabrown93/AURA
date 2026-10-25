package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_11_to_12 removes the season poster and title card types from saved movie and collection sets, they only have posters and backdrops
func migrate_11_to_12(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v11 to v12", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 11).Int("To Version", 12).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 11, 12)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	// Saved movie and collection sets
	result, err := conn.ExecContext(ctx, `
		UPDATE SavedItems
		SET season_poster_selected = 0, special_season_poster_selected = 0, titlecard_selected = 0
		WHERE poster_set_id IN (
			SELECT id FROM PosterSets WHERE type IN ('movie', 'collection')
		)
		AND (season_poster_selected = 1 OR special_season_poster_selected = 1 OR titlecard_selected = 1)
	`)
	if err != nil {
		logAction.SetError("Failed to update SavedItems table for movie and collection sets", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	if updated, err := result.RowsAffected(); err == nil {
		logAction.AppendResult("saved_sets_fixed", updated)
	}

	// Failed downloads of movie and collection sets show their selected types too
	if _, err := conn.ExecContext(ctx, `
		UPDATE DownloadHistory
		SET selected_types = json_set(selected_types, '$.season_poster', json('false'), '$.special_season_poster', json('false'), '$.titlecard', json('false'))
		WHERE set_id IN (
			SELECT set_id FROM PosterSets WHERE type IN ('movie', 'collection')
		)
		AND json_valid(selected_types)
	`); err != nil {
		logAction.SetError("Failed to update DownloadHistory table for movie and collection sets", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v11.0 to v12.0 completed successfully")
	return logging.LogErrorInfo{}
}
