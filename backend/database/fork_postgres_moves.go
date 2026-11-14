package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
)

// MoveMediaItems stages every source before moving any destination, preserving overlapping move chains.
func (p *PostgresDB) MoveMediaItems(ctx context.Context, moves []MediaItemMove) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin media moves", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1096110659)"); err != nil {
		return postgresError("Failed to suspend queue claims during media moves", err)
	}
	staged := append([]MediaItemMove(nil), moves...)
	for i, move := range moves {
		library, edition := moveTarget(move)
		if library == move.OldLibrary && edition == move.OldEdition {
			continue
		}
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return postgresError("Failed to generate media-move staging key", err)
		}
		// PostgreSQL text cannot contain SQLite's NUL staging prefix.
		staging := "aura-move-" + hex.EncodeToString(nonce[:])
		var collision bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM MediaItems WHERE library_title=$1
UNION ALL SELECT 1 FROM SavedItems WHERE library_title=$1 UNION ALL SELECT 1 FROM IgnoredItems WHERE library_title=$1
UNION ALL SELECT 1 FROM DownloadHistory WHERE library_title=$1 UNION ALL SELECT 1 FROM DownloadQueueJobs WHERE library_title=$1)`, staging).Scan(&collision); err != nil {
			return postgresError("Failed to check move staging key", err)
		}
		if collision {
			return logging.LogErrorInfo{Message: "Generated media-move staging key already exists; retry the move"}
		}
		if err = postgresMoveMediaKey(ctx, tx, move.TMDB_ID, move.OldLibrary, move.OldEdition, staging, move.OldEdition); err != nil {
			return postgresError("Failed to stage media move", err)
		}
		staged[i].OldLibrary = staging
	}
	for _, move := range staged {
		library, edition := moveTarget(move)
		if library != move.OldLibrary || edition != move.OldEdition {
			if err = postgresMoveMediaKey(ctx, tx, move.TMDB_ID, move.OldLibrary, move.OldEdition, library, edition); err != nil {
				return postgresError("Failed to move media key", err)
			}
		}
		if !move.Found {
			if _, err = tx.ExecContext(ctx, "UPDATE MediaItems SET on_server=0 WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", move.TMDB_ID, library, edition); err != nil {
				return postgresError("Failed to mark moved media as unavailable", err)
			}
			continue
		}
		item := move.NewItem
		item.TMDB_ID, item.LibraryTitle, item.Edition = move.TMDB_ID, library, edition
		if _, err = postgresUpsertMedia(ctx, tx, item); err != nil {
			return postgresError("Failed to refresh moved media details", err)
		}
	}
	return postgresError("Failed to commit media moves", tx.Commit())
}

// postgresMoveMediaKey lets destination-owned links win collisions without discarding other saved sets.
func postgresMoveMediaKey(ctx context.Context, tx *sql.Tx, tmdbID, oldLibrary, oldEdition, newLibrary, newEdition string) error {
	args := []any{tmdbID, oldLibrary, oldEdition, newLibrary, newEdition}
	queries := []string{
		`INSERT INTO SavedItems
(tmdb_id,library_title,edition,poster_set_id,poster_selected,backdrop_selected,season_poster_selected,special_season_poster_selected,titlecard_selected,autodownload,auto_add_new_collection_items,no_auto_upgrade,force_preload_missing,last_downloaded)
SELECT tmdb_id,$4,$5,poster_set_id,poster_selected,backdrop_selected,season_poster_selected,special_season_poster_selected,titlecard_selected,autodownload,auto_add_new_collection_items,no_auto_upgrade,force_preload_missing,last_downloaded
FROM SavedItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 ON CONFLICT DO NOTHING`,
		`DELETE FROM SavedItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 AND (library_title<>$4 OR edition<>$5)`,
		`INSERT INTO IgnoredItems (tmdb_id,library_title,edition,mode,current_sets)
SELECT tmdb_id,$4,$5,mode,current_sets FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 ON CONFLICT DO NOTHING`,
		`DELETE FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 AND (library_title<>$4 OR edition<>$5)`,
		`UPDATE DownloadQueueJobs SET library_title=$4,edition=$5 WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3`,
		`UPDATE DownloadHistory SET library_title=$4,edition=$5 WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3`,
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	var destinationID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM MediaItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", tmdbID, newLibrary, newEdition).Scan(&destinationID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, "UPDATE MediaItems SET library_title=$4,edition=$5 WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", args...)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM MediaItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 AND id<>$4", tmdbID, oldLibrary, oldEdition, destinationID)
	return err
}

// ReconcileMediaItemEdition retains upstream collision behavior: conflicting editions fail atomically.
func (p *PostgresDB) ReconcileMediaItemEdition(ctx context.Context, tmdbID, library, oldEdition string, item models.MediaItem) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin edition reconciliation", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE MediaItems SET edition=$1,rating_key=$2,type=$3,title=$4,year=$5 WHERE tmdb_id=$6 AND library_title=$7 AND edition=$8`,
		item.Edition, item.RatingKey, item.Type, item.Title, item.Year, tmdbID, library, oldEdition); err != nil {
		return postgresError("Failed to update media edition", err)
	}
	// SavedItems follows its ON UPDATE CASCADE; IgnoredItems intentionally has no parent foreign key.
	if _, err = tx.ExecContext(ctx, "UPDATE IgnoredItems SET edition=$1 WHERE tmdb_id=$2 AND library_title=$3 AND edition=$4", item.Edition, tmdbID, library, oldEdition); err != nil {
		return postgresError("Failed to reconcile ignored edition", err)
	}
	return postgresError("Failed to commit edition reconciliation", tx.Commit())
}

var _ DB = (*PostgresDB)(nil)
