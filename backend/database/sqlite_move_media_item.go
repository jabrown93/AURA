package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"fmt"
)

// MediaItemMove is a saved media item that moves to the item on a new main media server
type MediaItemMove struct {
	TMDB_ID    string
	OldLibrary string
	OldEdition string
	NewItem    models.MediaItem // The item on the new main media server, with its full details. Only the library is set if it wasn't found.
	Found      bool
}

// MoveMediaItem moves one saved media item. See MoveMediaItems.
func (s *SQliteDB) MoveMediaItem(ctx context.Context, tmdbID, oldLibrary, oldEdition string, newItem models.MediaItem, found bool) (Err logging.LogErrorInfo) {
	return s.MoveMediaItems(ctx, []MediaItemMove{{TMDB_ID: tmdbID, OldLibrary: oldLibrary, OldEdition: oldEdition, NewItem: newItem, Found: found}})
}

// MoveMediaItems moves saved media items to the items on a new main media server, all or none of them.
// Their saved sets, ignored status, queued downloads and download history move with them.
// If the new item is already saved, the two are merged and the saved sets of both are kept.
// An item that isn't on the new main media server only changes library and is marked as not on the server.
func (s *SQliteDB) MoveMediaItems(ctx context.Context, moves []MediaItemMove) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Moving %d MediaItems", len(moves)), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("Database connection is nil", "", nil)
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	for _, move := range moves {
		if Err := moveMediaItem(ctx, tx, move); Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return Err
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

func moveMediaItem(ctx context.Context, tx *sql.Tx, move MediaItemMove) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Moving MediaItem %s | %s | %s", move.TMDB_ID, move.OldLibrary, move.OldEdition), logging.LevelDebug)
	defer logAction.Complete()

	newItem := move.NewItem
	newLibrary, newEdition := newItem.LibraryTitle, newItem.Edition
	if newLibrary == "" {
		newLibrary, newEdition = move.OldLibrary, move.OldEdition
	} else if !move.Found {
		newEdition = move.OldEdition
	}

	if newLibrary != move.OldLibrary || newEdition != move.OldEdition {
		if err := moveMediaItemKey(ctx, tx, move.TMDB_ID, move.OldLibrary, move.OldEdition, newLibrary, newEdition); err != nil {
			logAction.SetError("Failed to move the media item", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	if !move.Found {
		if _, err := tx.ExecContext(ctx, `UPDATE MediaItems SET on_server = 0 WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`,
			move.TMDB_ID, newLibrary, newEdition); err != nil {
			logAction.SetError("Failed to mark the media item as not on the server", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		return logging.LogErrorInfo{}
	}

	newItem.TMDB_ID, newItem.LibraryTitle, newItem.Edition = move.TMDB_ID, newLibrary, newEdition
	rowID, Err := upsertMediaItem(ctx, tx, newItem)
	if Err.Message != "" {
		return Err
	}
	switch newItem.Type {
	case "movie":
		if newItem.Movie != nil {
			return upsertMovie(ctx, tx, newItem, rowID)
		}
	case "show":
		if newItem.Series != nil {
			seriesRowID, Err := upsertSeries(ctx, tx, newItem, rowID)
			if Err.Message != "" {
				return Err
			}
			return reconcileSeasonsAndEpisodes(ctx, tx, newItem, seriesRowID)
		}
	}
	return logging.LogErrorInfo{}
}

// moveMediaItemKey changes the library and edition of a media item in every table.
// Foreign keys are not enforced on this connection, so each table is updated.
func moveMediaItemKey(ctx context.Context, tx *sql.Tx, tmdbID, oldLibrary, oldEdition, newLibrary, newEdition string) error {
	oldKey := []any{tmdbID, oldLibrary, oldEdition}
	set := func(query string) error {
		_, err := tx.ExecContext(ctx, query, append([]any{newLibrary, newEdition}, oldKey...)...)
		return err
	}

	// Saved sets and ignored status: the new item's own rows win if both have one
	for _, table := range []string{"SavedItems", "IgnoredItems"} {
		if err := set(fmt.Sprintf(`UPDATE OR IGNORE %s SET library_title = ?, edition = ? WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`, table)); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`, table), oldKey...); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}

	for _, table := range []string{"DownloadQueueJobs", "DownloadHistory"} {
		if err := set(fmt.Sprintf(`UPDATE %s SET library_title = ?, edition = ? WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`, table)); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
	}

	// The media item: merged into the new item if it is already saved, otherwise moved
	var newRowID, oldRowID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM MediaItems WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`, tmdbID, newLibrary, newEdition).Scan(&newRowID)
	switch {
	case err == sql.ErrNoRows:
		return set(`UPDATE MediaItems SET library_title = ?, edition = ? WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`)
	case err != nil:
		return fmt.Errorf("MediaItems: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM MediaItems WHERE tmdb_id = ? AND library_title = ? AND edition = ?;`, oldKey...).Scan(&oldRowID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("MediaItems: %w", err)
	}
	return deleteMediaItemRow(ctx, tx, oldRowID)
}

// deleteMediaItemRow deletes a MediaItems row and its movie, series, season and episode rows
func deleteMediaItemRow(ctx context.Context, tx *sql.Tx, rowID int64) error {
	queries := []string{
		`DELETE FROM Episodes WHERE season_id IN (SELECT id FROM Seasons WHERE series_id IN (SELECT id FROM Series WHERE media_item_id = ?));`,
		`DELETE FROM Seasons WHERE series_id IN (SELECT id FROM Series WHERE media_item_id = ?);`,
		`DELETE FROM Series WHERE media_item_id = ?;`,
		`DELETE FROM Movies WHERE media_item_id = ?;`,
		`DELETE FROM MediaItems WHERE id = ?;`,
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query, rowID); err != nil {
			return fmt.Errorf("deleting media item %d: %w", rowID, err)
		}
	}
	return nil
}

// CountActiveDownloadQueueJobs returns how many download queue jobs are waiting or running
func (s *SQliteDB) CountActiveDownloadQueueJobs(ctx context.Context) (count int, Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx, "Counting active download queue jobs", logging.LevelTrace)
	defer logAction.Complete()

	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM DownloadQueueJobs WHERE status IN ('pending', 'processing');`).Scan(&count); err != nil {
		logAction.SetError("Failed to count download queue jobs", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}
	return count, logging.LogErrorInfo{}
}
