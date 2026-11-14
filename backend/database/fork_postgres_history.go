package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// InsertDownloadHistoryEntry keeps one entry per item/set and preserves retry count and source.
func (p *PostgresDB) InsertDownloadHistoryEntry(ctx context.Context, entry DownloadHistoryEntry) logging.LogErrorInfo {
	if entry.FailedImages == nil {
		entry.FailedImages = []models.ImageDownloadResult{}
	}
	failedJSON, err := json.Marshal(entry.FailedImages)
	if err != nil {
		return postgresError("Failed to encode failed images", err)
	}
	selectedJSON, err := json.Marshal(entry.SelectedTypes)
	if err != nil {
		return postgresError("Failed to encode history selection", err)
	}
	status := entry.Status
	if status == "" {
		switch {
		case entry.ImagesFailed == 0:
			status = "success"
		case entry.ImagesSucceeded == 0:
			status = "error"
		default:
			status = "warning"
		}
	}
	// SQLite serializes this read-then-update too; retain imported IDs without adding a dedup constraint.
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin history update", err)
	}
	defer tx.Rollback()
	var id int64
	var retries int
	findErr := tx.QueryRowContext(ctx, "SELECT id,retry_count FROM DownloadHistory WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 AND set_id=$4", entry.TMDB_ID, entry.LibraryTitle, entry.Edition, entry.SetID).Scan(&id, &retries)
	if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
		return postgresError("Failed to find history entry", findErr)
	}
	var delay any
	if entry.SetID != "" && retries >= 0 && retries < len(DownloadRetryDelays) {
		delay = int64(DownloadRetryDelays[retries].Seconds())
	}
	if findErr == nil {
		_, err = tx.ExecContext(ctx, `UPDATE DownloadHistory SET media_item_title=$1,media_item_year=$2,set_title=$3,set_creator=$4,selected_types=$5,warning_count=$6,
images_succeeded=$7,images_failed=$8,status=$9,failed_images=$10,created_at=CURRENT_TIMESTAMP,
next_retry_at=CASE WHEN $11::bigint IS NULL THEN NULL ELSE CURRENT_TIMESTAMP+$11::bigint*INTERVAL '1 second' END,
source=CASE WHEN $12='retry' AND source<>'' THEN source ELSE $12 END WHERE id=$13`,
			entry.MediaItemTitle, entry.MediaItemYear, entry.SetTitle, entry.SetCreator, string(selectedJSON), entry.WarningCount, entry.ImagesSucceeded, entry.ImagesFailed, status, string(failedJSON), delay, entry.Source, id)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO DownloadHistory
(tmdb_id,library_title,edition,media_item_title,media_item_year,set_id,set_title,set_creator,selected_types,warning_count,images_succeeded,images_failed,status,failed_images,next_retry_at,source)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,CASE WHEN $15::bigint IS NULL THEN NULL ELSE CURRENT_TIMESTAMP+$15::bigint*INTERVAL '1 second' END,$16)`,
			entry.TMDB_ID, entry.LibraryTitle, entry.Edition, entry.MediaItemTitle, entry.MediaItemYear, entry.SetID, entry.SetTitle, entry.SetCreator, string(selectedJSON), entry.WarningCount, entry.ImagesSucceeded, entry.ImagesFailed, status, string(failedJSON), delay, entry.Source)
	}
	if err != nil {
		return postgresError("Failed to save download history", err)
	}
	return postgresError("Failed to commit download history", tx.Commit())
}

// postgresHistoryRows shares the upstream scanner, whose column and model contracts are unchanged.
func (p *PostgresDB) postgresHistoryRows(ctx context.Context, suffix string, args ...any) ([]DownloadHistoryEntry, logging.LogErrorInfo) {
	entries := []DownloadHistoryEntry{}
	rows, err := p.conn.QueryContext(ctx, "SELECT "+downloadHistoryColumns+" FROM DownloadHistory "+suffix, args...)
	if err != nil {
		return entries, postgresError("Failed to query download history", err)
	}
	defer rows.Close()
	for rows.Next() {
		entry, err := scanDownloadHistoryEntry(rows)
		if err != nil {
			return entries, postgresError("Failed to scan download history", err)
		}
		entries = append(entries, entry)
	}
	return entries, postgresError("Failed to read download history", rows.Err())
}

// GetDownloadHistory sorts equal timestamps by ID to keep pagination stable.
func (p *PostgresDB) GetDownloadHistory(ctx context.Context, limit, offset int) ([]DownloadHistoryEntry, int, logging.LogErrorInfo) {
	var total int
	if err := p.conn.QueryRowContext(ctx, "SELECT count(*) FROM DownloadHistory").Scan(&total); err != nil {
		return nil, 0, postgresError("Failed to count download history", err)
	}
	var limitArg any = limit
	if limit < 0 {
		limitArg = nil
	}
	if offset < 0 {
		offset = 0
	}
	entries, info := p.postgresHistoryRows(ctx, "ORDER BY created_at DESC,id DESC LIMIT $1 OFFSET $2", limitArg, offset)
	return entries, total, info
}

// GetDownloadHistoryEntry distinguishes absence from database failure.
func (p *PostgresDB) GetDownloadHistoryEntry(ctx context.Context, id int64) (DownloadHistoryEntry, bool, logging.LogErrorInfo) {
	entry, err := scanDownloadHistoryEntry(p.conn.QueryRowContext(ctx, "SELECT "+downloadHistoryColumns+" FROM DownloadHistory WHERE id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return entry, false, logging.LogErrorInfo{}
	}
	if err != nil {
		return entry, false, postgresError("Failed to read history entry", err)
	}
	return entry, true, logging.LogErrorInfo{}
}

// GetDueDownloadRetries compares server timestamps rather than timezone-formatted strings.
func (p *PostgresDB) GetDueDownloadRetries(ctx context.Context) ([]DownloadHistoryEntry, logging.LogErrorInfo) {
	return p.postgresHistoryRows(ctx, "WHERE set_id<>'' AND next_retry_at<=CURRENT_TIMESTAMP ORDER BY next_retry_at,id")
}

// MarkDownloadHistoryRetried clears scheduling until the retried download fails again.
func (p *PostgresDB) MarkDownloadHistoryRetried(ctx context.Context, id int64) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "UPDATE DownloadHistory SET retry_count=retry_count+1,next_retry_at=NULL WHERE id=$1", id)
	return postgresError("Failed to count download retry", err)
}

// StopDownloadRetries keeps failure history while disabling automatic retries.
func (p *PostgresDB) StopDownloadRetries(ctx context.Context, id int64) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "UPDATE DownloadHistory SET next_retry_at=NULL WHERE id=$1", id)
	return postgresError("Failed to stop download retries", err)
}

// DeleteDownloadHistoryEntry removes one explicitly selected history row.
func (p *PostgresDB) DeleteDownloadHistoryEntry(ctx context.Context, id int64) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM DownloadHistory WHERE id=$1", id)
	return postgresError("Failed to delete history entry", err)
}

// DeleteAllDownloadHistory clears history without touching event statistics.
func (p *PostgresDB) DeleteAllDownloadHistory(ctx context.Context) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM DownloadHistory")
	return postgresError("Failed to clear download history", err)
}

// DeleteDownloadHistoryForSet also clears failures recorded before any set was chosen.
func (p *PostgresDB) DeleteDownloadHistoryForSet(ctx context.Context, tmdbID, library, edition, setID string) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM DownloadHistory WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3 AND (set_id=$4 OR set_id='')", tmdbID, library, edition, setID)
	return postgresError("Failed to clear history for downloaded set", err)
}
