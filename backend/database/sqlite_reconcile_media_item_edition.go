package database

import (
	"aura/logging"
	"aura/models"
	"context"
)

// ReconcileMediaItemEdition updates a MediaItems row
// This handles the case where a media server (e.g. Plex) starts reporting an Edition for an item
// that was already saved under no edition (or a different edition)
func (s *SQliteDB) ReconcileMediaItemEdition(ctx context.Context, tmdbID, libraryTitle, oldEdition string, updatedItem models.MediaItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Reconciling MediaItem Edition change", logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	if s == nil || s.conn == nil {
		logAction.SetError("Database connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Update the MediaItems row itself
	_, err = tx.ExecContext(ctx, `
        UPDATE MediaItems
        SET edition = ?, rating_key = ?, type = ?, title = ?, year = ?
        WHERE tmdb_id = ? AND library_title = ? AND edition = ?;
    `,
		updatedItem.Edition,
		updatedItem.RatingKey,
		updatedItem.Type,
		updatedItem.Title,
		updatedItem.Year,
		tmdbID,
		libraryTitle,
		oldEdition,
	)
	if err != nil {
		logAction.SetError("Failed to update MediaItems edition", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	// Carry any SavedItems rows over to the new edition (foreign key enforcement is not enabled
	// on this connection, so this can't rely on ON UPDATE CASCADE)
	_, err = tx.ExecContext(ctx, `
        UPDATE SavedItems SET edition = ? WHERE tmdb_id = ? AND library_title = ? AND edition = ?;
    `, updatedItem.Edition, tmdbID, libraryTitle, oldEdition)
	if err != nil {
		logAction.SetError("Failed to update SavedItems edition", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	// Carry any IgnoredItems row over to the new edition
	_, err = tx.ExecContext(ctx, `
        UPDATE IgnoredItems SET edition = ? WHERE tmdb_id = ? AND library_title = ? AND edition = ?;
    `, updatedItem.Edition, tmdbID, libraryTitle, oldEdition)
	if err != nil {
		logAction.SetError("Failed to update IgnoredItems edition", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().
		Str("tmdb_id", tmdbID).
		Str("library_title", libraryTitle).
		Str("old_edition", oldEdition).
		Str("new_edition", updatedItem.Edition).
		Msg("Reconciled MediaItem edition change")

	return Err
}
