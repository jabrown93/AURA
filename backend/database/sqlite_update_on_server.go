package database

import (
	"aura/logging"
	"context"
	"encoding/json"
)

func (s *SQliteDB) UpdateMediaItemOnServer(ctx context.Context, tmdbID string, libraryTitle string, onServer bool) (logErr logging.LogErrorInfo) {

	logErr = logging.LogErrorInfo{}

	// Start a transaction
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, "Updating MediaItem on_server flag in SQLite database", logging.LevelDebug)
		defer logAction.Complete()
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Update the on_server flag for the specified MediaItem
	_, err = tx.ExecContext(ctx, `
		UPDATE MediaItems
		SET on_server = ?
		WHERE tmdb_id = ? AND library_title = ?;
	`, onServer, tmdbID, libraryTitle)
	if err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, "Updating MediaItem on_server flag in SQLite database", logging.LevelDebug)
		defer logAction.Complete()
		logAction.SetError("Failed to update MediaItem on_server flag", "", map[string]any{"error": err.Error(), "tmdb_id": tmdbID, "library_title": libraryTitle, "on_server": onServer})
		return *logAction.Error
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, "Updating MediaItem on_server flag in SQLite database", logging.LevelDebug)
		defer logAction.Complete()
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logErr
}

func (s *SQliteDB) UpdateMediaItemsOnServer(ctx context.Context, libraryTitle string, tmdbIDs []string, onServer bool) logging.LogErrorInfo {
	if len(tmdbIDs) == 0 {
		return logging.LogErrorInfo{}
	}
	encodedIDs, err := json.Marshal(tmdbIDs)
	if err != nil {
		return logging.LogErrorInfo{Message: "Failed to encode media item IDs", Detail: map[string]any{"error": err.Error()}}
	}
	if _, err := s.conn.ExecContext(ctx, `
		UPDATE MediaItems
		SET on_server = ?
		WHERE library_title = ?
		  AND tmdb_id IN (SELECT value FROM json_each(?))
	`, onServer, libraryTitle, string(encodedIDs)); err != nil {
		return logging.LogErrorInfo{Message: "Failed to bulk-update MediaItem on_server flags", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}
