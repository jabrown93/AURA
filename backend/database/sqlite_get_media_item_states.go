package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"strings"
)

type MediaItemKey struct {
	TMDBID       string
	LibraryTitle string
}

type MediaItemState struct {
	Ignored    bool
	IgnoreMode string
	SavedSets  []models.DBSavedSet
}

func (s *SQliteDB) GetAllMediaItemStates(ctx context.Context) (map[MediaItemKey]MediaItemState, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Retrieving media item database states", logging.LevelDebug)
	defer logAction.Complete()

	states := make(map[MediaItemKey]MediaItemState)
	rows, err := s.conn.QueryContext(ctx, `
		SELECT DISTINCT
		       COALESCE(m.tmdb_id, i.tmdb_id, si.tmdb_id) AS tmdb_id,
		       COALESCE(m.library_title, i.library_title, si.library_title) AS library_title,
		       i.mode, ps.set_id, ps.user,
		       si.poster_selected, si.backdrop_selected, si.season_poster_selected,
		       si.special_season_poster_selected, si.titlecard_selected
		FROM MediaItems m
		FULL OUTER JOIN IgnoredItems i
		  ON i.tmdb_id = m.tmdb_id AND i.library_title = m.library_title
		FULL OUTER JOIN SavedItems si
		  ON si.tmdb_id = COALESCE(m.tmdb_id, i.tmdb_id)
		 AND si.library_title = COALESCE(m.library_title, i.library_title)
		LEFT JOIN PosterSets ps ON ps.id = si.poster_set_id
		ORDER BY library_title, tmdb_id, ps.set_id
	`)
	if err != nil {
		logAction.SetError("Failed to query media item database states", "", map[string]any{"error": err.Error()})
		return states, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var tmdbID, libraryTitle string
		var ignoreMode, setID, user sql.NullString
		var poster, backdrop, seasonPoster, specialSeasonPoster, titlecard sql.NullInt64
		if err := rows.Scan(
			&tmdbID, &libraryTitle, &ignoreMode, &setID, &user,
			&poster, &backdrop, &seasonPoster, &specialSeasonPoster, &titlecard,
		); err != nil {
			logAction.SetError("Failed to scan media item database state", "", map[string]any{"error": err.Error()})
			return states, *logAction.Error
		}

		key := MediaItemKey{TMDBID: tmdbID, LibraryTitle: libraryTitle}
		state := states[key]
		trimmedIgnoreMode := strings.TrimSpace(ignoreMode.String)
		if ignoreMode.Valid && trimmedIgnoreMode != "" {
			state.Ignored = true
			state.IgnoreMode = trimmedIgnoreMode
		}
		if setID.Valid {
			state.SavedSets = append(state.SavedSets, models.DBSavedSet{
				ID:          setID.String,
				UserCreated: user.String,
				SelectedTypes: models.SelectedTypes{
					Poster:              poster.Int64 == 1,
					Backdrop:            backdrop.Int64 == 1,
					SeasonPoster:        seasonPoster.Int64 == 1,
					SpecialSeasonPoster: specialSeasonPoster.Int64 == 1,
					Titlecard:           titlecard.Int64 == 1,
				},
			})
		}
		states[key] = state
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while reading media item database states", "", map[string]any{"error": err.Error()})
		return states, *logAction.Error
	}
	return states, logging.LogErrorInfo{}
}
