package database

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"context"
	"strings"
)

func (s *SQliteDB) GetTempIgnoredItems(ctx context.Context) (items []models.MediaItem, Err logging.LogErrorInfo) {
	Err = logging.LogErrorInfo{}
	if s == nil || s.conn == nil {
		return nil, logging.LogErrorInfo{Message: "Database connection is nil"}
	}

	// Query the database for temp ignored items
	rows, err := s.conn.QueryContext(ctx, `
        SELECT tmdb_id, library_title, edition, mode, current_sets
        FROM IgnoredItems
        WHERE mode = 'until-set-available' OR mode = 'until-new-set-available';
    `)
	if err != nil {
		return nil, logging.LogErrorInfo{
			Message: "Failed to get temp ignored items",
			Detail:  map[string]any{"error": err.Error()},
		}
	}
	defer rows.Close()

	var tmdbID string
	var libraryTitle string
	var edition string
	var mode string
	var currentSets string
	for rows.Next() {
		if err := rows.Scan(&tmdbID, &libraryTitle, &edition, &mode, &currentSets); err != nil {
			return nil, logging.LogErrorInfo{
				Message: "Failed to scan temp ignored item",
				Detail:  map[string]any{"error": err.Error()},
			}
		}

		// Get the Media Item from the cache
		cachedItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(libraryTitle, tmdbID, edition)
		if !found {
			logging.LOGGER.Warn().Timestamp().
				Str("tmdb_id", tmdbID).
				Str("library_title", libraryTitle).
				Msg("Temp ignored item not found in cache")
			continue
		}
		cachedItem.IgnoredMode = mode
		cachedItem.IgnoredSets = strings.Split(currentSets, ",")
		items = append(items, *cachedItem)
	}
	if err := rows.Err(); err != nil {
		return nil, logging.LogErrorInfo{
			Message: "Failed while iterating over temp ignored items",
			Detail:  map[string]any{"error": err.Error()},
		}
	}

	return items, Err
}

func (s *SQliteDB) IgnoreMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, mode, currentSets string) (Err logging.LogErrorInfo) {
	Err = logging.LogErrorInfo{}

	if s == nil || s.conn == nil {
		return logging.LogErrorInfo{Message: "Database connection is nil"}
	}

	tmdbID = strings.TrimSpace(tmdbID)
	libraryTitle = strings.TrimSpace(libraryTitle)
	edition = strings.TrimSpace(edition)
	mode = strings.ToLower(strings.TrimSpace(mode))

	if tmdbID == "" || libraryTitle == "" {
		return logging.LogErrorInfo{
			Message: "tmdb_id and library_title are required",
			Detail:  map[string]any{"tmdb_id": tmdbID, "library_title": libraryTitle},
		}
	}

	if mode != "always" && mode != "until-set-available" && mode != "until-new-set-available" {
		return logging.LogErrorInfo{
			Message: "Invalid ignore mode",
			Detail:  map[string]any{"mode": mode, "valid_modes": []string{"always", "until-set-available", "until-new-set-available"}},
		}
	} else if mode == "until-new-set-available" && currentSets == "" {
		return logging.LogErrorInfo{
			Message: "current_sets is required for 'until-new-set-available' mode",
			Detail:  map[string]any{"mode": mode, "current_sets": currentSets},
		}
	}

	// Determine insert vs update for logging
	var existed int
	_ = s.conn.QueryRowContext(ctx, `
        SELECT 1
        FROM IgnoredItems
        WHERE tmdb_id = ? AND library_title = ? AND edition = ?
        LIMIT 1;
    `, tmdbID, libraryTitle, edition).Scan(&existed)
	op := "INSERT"
	if existed == 1 {
		op = "UPDATE"
	}

	_, err := s.conn.ExecContext(ctx, `
        INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode, current_sets)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(tmdb_id, library_title, edition) DO UPDATE SET
            mode = excluded.mode,
            current_sets = excluded.current_sets;
    `, tmdbID, libraryTitle, edition, mode, currentSets)
	if err != nil {
		return logging.LogErrorInfo{
			Message: "Failed to ignore media item",
			Detail:  map[string]any{"error": err.Error(), "tmdb_id": tmdbID, "library_title": libraryTitle, "edition": edition, "mode": mode, "current_sets": currentSets},
		}
	}

	logging.LOGGER.Debug().Timestamp().
		Str("op", op).
		Str("table", "IgnoredItems").
		Str("tmdb_id", tmdbID).
		Str("library_title", libraryTitle).
		Str("edition", edition).
		Str("mode", mode).
		Str("current_sets", currentSets).
		Msg("Ignored media item")

	return Err
}
