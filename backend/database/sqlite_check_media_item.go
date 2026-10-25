package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"strings"
)

func (s *SQliteDB) CheckIfMediaItemExists(ctx context.Context, TMDB_ID, libraryTitle, edition string) (ignored bool, ignoreMode string, sets []models.DBSavedSet, logErr logging.LogErrorInfo) {
	ignored = false
	ignoreMode = ""
	sets = []models.DBSavedSet{}
	logErr = logging.LogErrorInfo{}

	if s.conn == nil {
		return ignored, ignoreMode, sets, logErr
	}

	// 0) Check if IgnoredItems table exists
	{
		var tableName string
		err := s.conn.QueryRowContext(ctx, `
			SELECT name
			FROM sqlite_master
			WHERE type='table' AND name='IgnoredItems';
		`).Scan(&tableName)
		if err != nil {
			if err != sql.ErrNoRows {
				_, logAction := logging.AddSubActionToContext(ctx, "Checking IgnoredItems table existence", logging.LevelError)
				defer logAction.Complete()
				logAction.SetError("Failed to query database for IgnoredItems table", err.Error(), map[string]any{
					"error": err.Error(),
				})
				return ignored, ignoreMode, sets, *logAction.Error
			}
			// Table does not exist, so no ignored items
			return ignored, ignoreMode, sets, logErr
		}
	}

	// 1) Check ignore status
	{
		var mode sql.NullString
		err := s.conn.QueryRowContext(ctx, `
            SELECT mode
            FROM IgnoredItems
            WHERE tmdb_id = ?
              AND library_title = ?
              AND edition = ?
            LIMIT 1;
        `, TMDB_ID, libraryTitle, edition).Scan(&mode)

		if err != nil && err != sql.ErrNoRows {
			_, logAction := logging.AddSubActionToContext(ctx, "Checking ignored status for media item", logging.LevelError)
			defer logAction.Complete()
			logAction.SetError("Failed to query database for ignored status", err.Error(), map[string]any{
				"error":        err.Error(),
				"TMDB_ID":      TMDB_ID,
				"libraryTitle": libraryTitle,
			})
			return ignored, ignoreMode, sets, *logAction.Error
		}

		if err == nil && mode.Valid && strings.TrimSpace(mode.String) != "" {
			ignored = true
			ignoreMode = strings.TrimSpace(mode.String)
		}
	}

	// If ignored, we don't care about saved sets.
	if ignored {
		return ignored, ignoreMode, sets, logErr
	}

	// 2) Fetch saved sets for this media item
	query := `
        SELECT DISTINCT
            ps.set_id,
            ps.user,
            si.poster_selected,
            si.backdrop_selected,
            si.season_poster_selected,
            si.special_season_poster_selected,
            si.titlecard_selected
        FROM SavedItems si
        JOIN PosterSets ps ON ps.id = si.poster_set_id
        WHERE si.tmdb_id = ?
          AND si.library_title = ?
          AND si.edition = ?;
    `
	rows, err := s.conn.QueryContext(ctx, query, TMDB_ID, libraryTitle, edition)
	if err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, "Checking if media item exists in database", logging.LevelError)
		defer logAction.Complete()
		logAction.SetError("Failed to query database for media item", err.Error(), map[string]any{
			"error":        err.Error(),
			"query":        query,
			"TMDB_ID":      TMDB_ID,
			"libraryTitle": libraryTitle,
		})
		return ignored, ignoreMode, sets, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var set models.DBSavedSet
		var posterSelected, backdropSelected, seasonPosterSelected, specialSeasonPosterSelected, titlecardSelected int
		if err := rows.Scan(&set.ID, &set.UserCreated, &posterSelected, &backdropSelected, &seasonPosterSelected, &specialSeasonPosterSelected, &titlecardSelected); err != nil {
			_, logAction := logging.AddSubActionToContext(ctx, "Scanning media item row", logging.LevelError)
			defer logAction.Complete()
			logAction.SetError("Failed to scan media item row", err.Error(), map[string]any{
				"error": err.Error(),
				"query": query,
			})
			return ignored, ignoreMode, sets, *logAction.Error
		}

		set.SelectedTypes = models.SelectedTypes{
			Poster:              posterSelected == 1,
			Backdrop:            backdropSelected == 1,
			SeasonPoster:        seasonPosterSelected == 1,
			SpecialSeasonPoster: specialSeasonPosterSelected == 1,
			Titlecard:           titlecardSelected == 1,
		}

		sets = append(sets, set)
	}

	if err := rows.Err(); err != nil {
		_, logAction := logging.AddSubActionToContext(ctx, "Finalizing media item rows", logging.LevelError)
		defer logAction.Complete()
		logAction.SetError("Error occurred during rows iteration", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return ignored, ignoreMode, sets, *logAction.Error
	}

	return ignored, ignoreMode, sets, logErr
}
