package database

import (
	"aura/logging"
	"aura/models"
	"context"
)

func (s *SQliteDB) UpdateMediaItem(ctx context.Context, updatedItem models.MediaItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Updating MediaItem in database", logging.LevelDebug)
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

	res, err := tx.ExecContext(ctx, `
        UPDATE MediaItems
        SET rating_key = ?, type = ?, title = ?, year = ?
        WHERE tmdb_id = ? AND library_title = ? AND edition = ?;
    `,
		updatedItem.RatingKey,
		updatedItem.Type,
		updatedItem.Title,
		updatedItem.Year,
		updatedItem.TMDB_ID,
		updatedItem.LibraryTitle,
		updatedItem.Edition,
	)
	if err != nil {
		logAction.SetError("Failed to execute update statement", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	affected, _ := res.RowsAffected()
	logging.LOGGER.Info().Timestamp().
		Str("op", "UPDATE").
		Str("table", "MediaItems").
		Int64("rows", affected).
		Str("tmdb_id", updatedItem.TMDB_ID).
		Str("library_title", updatedItem.LibraryTitle).
		Msg("Updated media item")

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return Err
}
