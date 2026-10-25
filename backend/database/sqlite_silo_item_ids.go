package database

import (
	"aura/logging"
	"context"
	"fmt"
	"time"
)

// SiloItemIDs is the provider IDs of a Silo catalog item.
// Silo's catalog listing does not include provider IDs, so they are saved here to avoid
// getting the details of every item from Silo after each restart.
type SiloItemIDs struct {
	ContentID    string    `json:"content_id"`
	TMDB_ID      string    `json:"tmdb_id"`
	IMDB_ID      string    `json:"imdb_id"`
	TVDB_ID      string    `json:"tvdb_id"`
	FirstAirDate string    `json:"first_air_date"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *SQliteDB) GetAllSiloItemIDs(ctx context.Context) (items []SiloItemIDs, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get All Silo Item IDs", logging.LevelDebug)
	defer logAction.Complete()

	items = []SiloItemIDs{}

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return items, *logAction.Error
	}

	rows, err := s.conn.QueryContext(ctx, `
SELECT content_id, tmdb_id, imdb_id, tvdb_id, first_air_date, updated_at
FROM SiloItemIDs;
`)
	if err != nil {
		logAction.SetError("DB: SELECT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var item SiloItemIDs
		if err := rows.Scan(&item.ContentID, &item.TMDB_ID, &item.IMDB_ID, &item.TVDB_ID, &item.FirstAirDate, &item.UpdatedAt); err != nil {
			logAction.SetError("DB: scan SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
			return items, *logAction.Error
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("DB: iterate SiloItemIDs rows failed", err.Error(), map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}

	logAction.AppendResult("count", len(items))
	return items, logging.LogErrorInfo{}
}

func (s *SQliteDB) UpsertSiloItemIDs(ctx context.Context, items []SiloItemIDs) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Upsert %d Silo Item IDs", len(items)), logging.LevelTrace)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("DB: TX BEGIN failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO SiloItemIDs (content_id, tmdb_id, imdb_id, tvdb_id, first_air_date, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(content_id) DO UPDATE SET
    tmdb_id = excluded.tmdb_id,
    imdb_id = excluded.imdb_id,
    tvdb_id = excluded.tvdb_id,
    first_air_date = excluded.first_air_date,
    updated_at = excluded.updated_at;
`)
	if err != nil {
		logAction.SetError("DB: prepare UPSERT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, item.ContentID, item.TMDB_ID, item.IMDB_ID, item.TVDB_ID, item.FirstAirDate, item.UpdatedAt); err != nil {
			logAction.SetError("DB: UPSERT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error(), "content_id": item.ContentID})
			return *logAction.Error
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("DB: TX COMMIT failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}
