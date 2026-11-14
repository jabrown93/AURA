package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// EnqueueDownloadQueueJob preserves the upstream payload and normalizes its image selections.
func (p *PostgresDB) EnqueueDownloadQueueJob(ctx context.Context, item models.DBSavedItem) (int64, logging.LogErrorInfo) {
	tmdbID, library, edition, title := item.QueueEntry()
	data, err := json.Marshal(item.WithSetImageTypes())
	if err != nil {
		return 0, postgresError("Failed to encode queue payload", err)
	}
	var id int64
	err = p.conn.QueryRowContext(ctx, "INSERT INTO DownloadQueueJobs (tmdb_id,library_title,edition,media_item_title,payload) VALUES ($1,$2,$3,$4,$5) RETURNING id", tmdbID, library, edition, title, string(data)).Scan(&id)
	return id, postgresError("Failed to enqueue download", err)
}

// ClaimNextDownloadQueueJob serializes claims so two waiting jobs for one item cannot start together.
func (p *PostgresDB) ClaimNextDownloadQueueJob(ctx context.Context, workerID string) (DownloadQueueJob, bool, logging.LogErrorInfo) {
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return DownloadQueueJob{}, false, postgresError("Failed to begin queue claim", err)
	}
	defer tx.Rollback()
	// SKIP LOCKED alone only excludes the chosen row, not other pending jobs for that item.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1096110659)"); err != nil {
		return DownloadQueueJob{}, false, postgresError("Failed to lock queue claim", err)
	}
	var job DownloadQueueJob
	var payload []byte
	err = tx.QueryRowContext(ctx, `WITH next AS (
SELECT waiting.id FROM DownloadQueueJobs waiting WHERE waiting.status='pending'
AND NOT EXISTS (SELECT 1 FROM DownloadQueueJobs running WHERE running.status='processing'
 AND running.tmdb_id=waiting.tmdb_id AND running.library_title=waiting.library_title AND running.edition=waiting.edition)
ORDER BY waiting.created_at,waiting.id LIMIT 1 FOR UPDATE OF waiting SKIP LOCKED)
UPDATE DownloadQueueJobs job SET status='processing',started_at=CURRENT_TIMESTAMP,worker_id=$1
FROM next WHERE job.id=next.id RETURNING job.id,job.tmdb_id,job.library_title,job.edition,job.media_item_title,job.payload,job.status,job.created_at,job.started_at`, workerID).Scan(
		&job.ID, &job.TMDB_ID, &job.LibraryTitle, &job.Edition, &job.MediaItemTitle, &payload, &job.Status, &job.CreatedAt, &job.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DownloadQueueJob{}, false, logging.LogErrorInfo{}
	}
	if err != nil {
		return DownloadQueueJob{}, false, postgresError("Failed to claim download job", err)
	}
	if err = json.Unmarshal(payload, &job.Payload); err != nil {
		return DownloadQueueJob{}, false, postgresError("Failed to decode claimed job", err)
	}
	if err = tx.Commit(); err != nil {
		return DownloadQueueJob{}, false, postgresError("Failed to commit queue claim", err)
	}
	return job, true, logging.LogErrorInfo{}
}

// FinishDownloadQueueJob always stores JSON arrays, including empty error and warning lists.
func (p *PostgresDB) FinishDownloadQueueJob(ctx context.Context, jobID int64, status, message string, errs, warnings []string) logging.LogErrorInfo {
	if errs == nil {
		errs = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	errorsJSON, err := json.Marshal(errs)
	if err != nil {
		return postgresError("Failed to encode queue errors", err)
	}
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return postgresError("Failed to encode queue warnings", err)
	}
	_, err = p.conn.ExecContext(ctx, `UPDATE DownloadQueueJobs SET status=$1,result_message=$2,result_errors=$3,result_warnings=$4,finished_at=CURRENT_TIMESTAMP WHERE id=$5`, status, message, string(errorsJSON), string(warningsJSON), jobID)
	return postgresError("Failed to finish download job", err)
}

// GetAllDownloadQueueJobs restores payload and result types without lossy JSON coercion.
func (p *PostgresDB) GetAllDownloadQueueJobs(ctx context.Context) ([]DownloadQueueJob, logging.LogErrorInfo) {
	jobs := []DownloadQueueJob{}
	rows, err := p.conn.QueryContext(ctx, `SELECT id,tmdb_id,library_title,edition,media_item_title,payload,status,result_message,result_errors,result_warnings,created_at,started_at,finished_at
FROM DownloadQueueJobs ORDER BY created_at,id`)
	if err != nil {
		return jobs, postgresError("Failed to query download queue", err)
	}
	defer rows.Close()
	for rows.Next() {
		var job DownloadQueueJob
		var payload, errorsJSON, warningsJSON []byte
		if err := rows.Scan(&job.ID, &job.TMDB_ID, &job.LibraryTitle, &job.Edition, &job.MediaItemTitle, &payload, &job.Status, &job.ResultMessage, &errorsJSON, &warningsJSON, &job.CreatedAt, &job.StartedAt, &job.FinishedAt); err != nil {
			return jobs, postgresError("Failed to scan queued download", err)
		}
		if err := json.Unmarshal(payload, &job.Payload); err != nil {
			return jobs, postgresError("Failed to decode queue payload", err)
		}
		if err := json.Unmarshal(errorsJSON, &job.ResultErrors); err != nil {
			return jobs, postgresError("Failed to decode queue errors", err)
		}
		if err := json.Unmarshal(warningsJSON, &job.ResultWarnings); err != nil {
			return jobs, postgresError("Failed to decode queue warnings", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, postgresError("Failed to read download queue", rows.Err())
}

// DeleteDownloadQueueJob retains upstream's explicit single-job deletion behavior.
func (p *PostgresDB) DeleteDownloadQueueJob(ctx context.Context, id int64) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM DownloadQueueJobs WHERE id=$1", id)
	return postgresError("Failed to delete queue job", err)
}

// DeleteAllDownloadQueueJobs preserves in-flight jobs.
func (p *PostgresDB) DeleteAllDownloadQueueJobs(ctx context.Context) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM DownloadQueueJobs WHERE status<>'processing'")
	return postgresError("Failed to clear queue", err)
}

// ResetInterruptedDownloadQueueJobs makes another worker's interrupted jobs retryable after restart.
func (p *PostgresDB) ResetInterruptedDownloadQueueJobs(ctx context.Context, workerID string) (int64, logging.LogErrorInfo) {
	result, err := p.conn.ExecContext(ctx, "UPDATE DownloadQueueJobs SET status='pending',started_at=NULL,worker_id='' WHERE status='processing' AND worker_id<>$1", workerID)
	if err != nil {
		return 0, postgresError("Failed to reset interrupted jobs", err)
	}
	count, err := result.RowsAffected()
	return count, postgresError("Failed to count reset jobs", err)
}

// CountActiveDownloadQueueJobs excludes completed history from the queue count.
func (p *PostgresDB) CountActiveDownloadQueueJobs(ctx context.Context) (int, logging.LogErrorInfo) {
	var count int
	err := p.conn.QueryRowContext(ctx, "SELECT count(*) FROM DownloadQueueJobs WHERE status IN ('pending','processing')").Scan(&count)
	return count, postgresError("Failed to count active jobs", err)
}

// MoveDownloadQueueJobToTop adjusts a pending job's sort timestamp without changing its identity.
func (p *PostgresDB) MoveDownloadQueueJobToTop(ctx context.Context, id int64) (bool, logging.LogErrorInfo) {
	result, err := p.conn.ExecContext(ctx, `UPDATE DownloadQueueJobs SET created_at=(SELECT min(created_at) FROM DownloadQueueJobs WHERE status='pending')-INTERVAL '1 second' WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		return false, postgresError("Failed to prioritize queue job", err)
	}
	count, err := result.RowsAffected()
	return count > 0, postgresError("Failed to read prioritized job count", err)
}

// MoveDownloadQueueJobAheadOfOthers preserves download-window order ahead of automatic jobs.
func (p *PostgresDB) MoveDownloadQueueJobAheadOfOthers(ctx context.Context, id int64) (bool, logging.LogErrorInfo) {
	result, err := p.conn.ExecContext(ctx, `UPDATE DownloadQueueJobs SET created_at=(SELECT min(created_at) FROM DownloadQueueJobs
WHERE status='pending' AND id<>$1 AND COALESCE(payload->>'source','')<>'download_window')-INTERVAL '1 second'
WHERE id=$1 AND status='pending' AND EXISTS (SELECT 1 FROM DownloadQueueJobs
WHERE status='pending' AND id<>$1 AND COALESCE(payload->>'source','')<>'download_window')`, id)
	if err != nil {
		return false, postgresError("Failed to prioritize download-window job", err)
	}
	count, err := result.RowsAffected()
	return count > 0, postgresError("Failed to read prioritized job count", err)
}
