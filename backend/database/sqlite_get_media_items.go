package database

import (
	"aura/logging"
	"aura/models"
	"context"
)

// GetAllMediaItems returns the saved and ignored media items
func (s *SQliteDB) GetAllMediaItems(ctx context.Context) (items []models.MediaItem, logErr logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Retrieving all MediaItems from SQLite database", logging.LevelDebug)
	defer logAction.Complete()

	items = []models.MediaItem{}
	logErr = logging.LogErrorInfo{}

	// Query the saved and ignored MediaItems
	// A single read does not need a transaction, which would take the write lock (immediate transactions)
	rows, err := s.conn.QueryContext(ctx, `
		SELECT tmdb_id, library_title, edition, rating_key, type, title, year
		FROM MediaItems mi
		WHERE NOT (`+notSavedOrIgnored+`);
	`)
	if err != nil {
		logAction.SetError("Failed to query MediaItems", "", map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}
	defer rows.Close()

	// Iterate through the rows
	for rows.Next() {
		var item models.MediaItem
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year); err != nil {
			logAction.SetError("Failed to scan MediaItem row", "", map[string]any{"error": err.Error()})
			return items, *logAction.Error
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while iterating over MediaItems rows", "", map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}

	return items, logErr
}

type MediaItemWithFlags struct {
	TMDB_ID      string
	LibraryTitle string
	Edition      string
	RatingKey    string
	Type         string
	Title        string
	Year         int
	HasSavedSet  bool
	IsIgnored    bool
}

func (s *SQliteDB) GetAllMediaItemsWithFlags(ctx context.Context) ([]MediaItemWithFlags, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Retrieving all MediaItems with set/ignored flags", logging.LevelDebug)
	defer logAction.Complete()

	items := []MediaItemWithFlags{}
	logErr := logging.LogErrorInfo{}

	// A single read does not need a transaction, which would take the write lock (immediate transactions)
	rows, err := s.conn.QueryContext(ctx, `
        SELECT
            m.tmdb_id,
            m.library_title,
            m.edition,
            m.rating_key,
            m.type,
            m.title,
            m.year,
            CASE WHEN s.tmdb_id IS NOT NULL THEN 1 ELSE 0 END AS has_saved_set,
            CASE WHEN i.tmdb_id IS NOT NULL THEN 1 ELSE 0 END AS is_ignored
        FROM
            MediaItems m
        LEFT JOIN
            SavedItems s
            ON m.tmdb_id = s.tmdb_id AND m.library_title = s.library_title AND m.edition = s.edition
        LEFT JOIN
            IgnoredItems i
            ON m.tmdb_id = i.tmdb_id AND m.library_title = i.library_title AND m.edition = i.edition
        GROUP BY
            m.tmdb_id, m.library_title, m.edition
    `)
	if err != nil {
		logAction.SetError("Failed to query MediaItems with flags", "", map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var item MediaItemWithFlags
		var hasSavedSet, isIgnored int
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year, &hasSavedSet, &isIgnored); err != nil {
			logAction.SetError("Failed to scan row", "", map[string]any{"error": err.Error()})
			return items, *logAction.Error
		}
		item.HasSavedSet = hasSavedSet == 1
		item.IsIgnored = isIgnored == 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while iterating over MediaItems rows", "", map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}

	return items, logErr
}
