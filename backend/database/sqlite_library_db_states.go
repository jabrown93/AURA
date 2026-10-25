package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// MediaItemDBState is an item's saved sets and ignored state in the database
type MediaItemDBState struct {
	Ignored     bool
	IgnoredMode string
	Sets        []models.DBSavedSet
}

func dbStateKey(tmdbID, edition string) string {
	return tmdbID + "|" + edition
}

// ApplyDBState sets an item's saved sets and ignored state from the states of its library
func ApplyDBState(states map[string]MediaItemDBState, item *models.MediaItem) {
	state := states[dbStateKey(item.TMDB_ID, item.Edition)]
	if state.Ignored {
		item.DBSavedSets = []models.DBSavedSet{}
		item.IgnoredInDB = true
		item.IgnoredMode = state.IgnoredMode
		return
	}
	item.DBSavedSets = state.Sets
	if item.DBSavedSets == nil {
		item.DBSavedSets = []models.DBSavedSet{}
	}
}

// GetLibraryDBStates returns the saved sets and ignored state of a library's items, by TMDB ID and edition
func (s *SQliteDB) GetLibraryDBStates(ctx context.Context, libraryTitle string) (map[string]MediaItemDBState, logging.LogErrorInfo) {
	states := map[string]MediaItemDBState{}
	fail := func(message string, err error) (map[string]MediaItemDBState, logging.LogErrorInfo) {
		return states, dbError(ctx, "Getting the saved sets and ignored items of a library", message, err)
	}

	ignoredRows, err := s.conn.QueryContext(ctx, `
		SELECT tmdb_id, edition, mode
		FROM IgnoredItems
		WHERE library_title = ?;
	`, libraryTitle)
	if err != nil {
		return fail("Failed to get the ignored items", err)
	}
	defer ignoredRows.Close()
	for ignoredRows.Next() {
		var tmdbID, edition, mode string
		if err := ignoredRows.Scan(&tmdbID, &edition, &mode); err != nil {
			return fail("Failed to read an ignored item", err)
		}
		if mode = strings.TrimSpace(mode); mode != "" {
			states[dbStateKey(tmdbID, edition)] = MediaItemDBState{Ignored: true, IgnoredMode: mode}
		}
	}
	if err := ignoredRows.Err(); err != nil {
		return fail("Failed to read the ignored items", err)
	}

	setRows, err := s.conn.QueryContext(ctx, `
		SELECT DISTINCT
			si.tmdb_id,
			si.edition,
			ps.set_id,
			ps.user,
			si.poster_selected,
			si.backdrop_selected,
			si.season_poster_selected,
			si.special_season_poster_selected,
			si.titlecard_selected
		FROM SavedItems si
		JOIN PosterSets ps ON ps.id = si.poster_set_id
		WHERE si.library_title = ?;
	`, libraryTitle)
	if err != nil {
		return fail("Failed to get the saved sets", err)
	}
	defer setRows.Close()
	for setRows.Next() {
		var tmdbID, edition string
		var set models.DBSavedSet
		var poster, backdrop, seasonPoster, specialSeasonPoster, titlecard int
		if err := setRows.Scan(&tmdbID, &edition, &set.ID, &set.UserCreated, &poster, &backdrop, &seasonPoster, &specialSeasonPoster, &titlecard); err != nil {
			return fail("Failed to read a saved set", err)
		}
		set.SelectedTypes = models.SelectedTypes{
			Poster:              poster == 1,
			Backdrop:            backdrop == 1,
			SeasonPoster:        seasonPoster == 1,
			SpecialSeasonPoster: specialSeasonPoster == 1,
			Titlecard:           titlecard == 1,
		}
		key := dbStateKey(tmdbID, edition)
		state := states[key]
		// An ignored item's saved sets are not shown
		if !state.Ignored {
			state.Sets = append(state.Sets, set)
			states[key] = state
		}
	}
	if err := setRows.Err(); err != nil {
		return fail("Failed to read the saved sets", err)
	}
	return states, logging.LogErrorInfo{}
}

// SaveLibraryItems saves a page of the main media server's items in one transaction
func (s *SQliteDB) SaveLibraryItems(ctx context.Context, items []models.MediaItem, seenAt int64) logging.LogErrorInfo {
	fail := func(message string, err error) logging.LogErrorInfo {
		return dbError(ctx, "Saving library items to the database", message, err)
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fail("Failed to start transaction", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO MediaItems (
			tmdb_id, library_title, edition, rating_key, type, title, year, on_server,
			added_at, updated_at, released_at, latest_episode_added_at, content_rating, summary, guids,
			first_seen_at, last_seen_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tmdb_id, library_title, edition) DO UPDATE SET
			rating_key              = excluded.rating_key,
			type                    = excluded.type,
			title                   = excluded.title,
			year                    = excluded.year,
			on_server               = 1,
			added_at                = excluded.added_at,
			updated_at              = excluded.updated_at,
			released_at             = excluded.released_at,
			latest_episode_added_at = excluded.latest_episode_added_at,
			content_rating          = excluded.content_rating,
			summary                 = excluded.summary,
			guids                   = excluded.guids,
			first_seen_at           = CASE WHEN MediaItems.first_seen_at = 0 THEN excluded.first_seen_at ELSE MediaItems.first_seen_at END,
			last_seen_at            = excluded.last_seen_at;
	`)
	if err != nil {
		return fail("Failed to prepare the library item save", err)
	}
	defer stmt.Close()
	for _, item := range items {
		if item.TMDB_ID == "" || (item.Type != "movie" && item.Type != "show") {
			continue
		}
		guids, err := json.Marshal(item.Guids)
		if err != nil || item.Guids == nil {
			guids = []byte("[]")
		}
		if _, err := stmt.ExecContext(ctx,
			item.TMDB_ID, item.LibraryTitle, item.Edition, item.RatingKey, item.Type, item.Title, item.Year,
			item.AddedAt, item.UpdatedAt, item.ReleasedAt, item.LatestEpisodeAddedAt, item.ContentRating, item.Summary, string(guids),
			seenAt, seenAt,
		); err != nil {
			return fail("Failed to save a library item", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fail("Failed to commit transaction", err)
	}
	return logging.LogErrorInfo{}
}

// notSavedOrIgnored is a condition for MediaItems rows that are only library items
const notSavedOrIgnored = `
	NOT EXISTS (SELECT 1 FROM SavedItems si WHERE si.tmdb_id = mi.tmdb_id AND si.library_title = mi.library_title AND si.edition = mi.edition)
	AND NOT EXISTS (SELECT 1 FROM IgnoredItems ii WHERE ii.tmdb_id = mi.tmdb_id AND ii.library_title = mi.library_title AND ii.edition = mi.edition)`

// FinishLibraryLoad removes gone library items and marks gone saved or ignored ones as not on the server
func (s *SQliteDB) FinishLibraryLoad(ctx context.Context, usedLibraries []string, loadedSince map[string]int64) logging.LogErrorInfo {
	fail := func(message string, err error) logging.LogErrorInfo {
		return dbError(ctx, "Removing library items that are no longer on the media server", message, err)
	}

	// gone matches the rows of items that are no longer on the media server
	gone := "mi.library_title NOT IN (SELECT value FROM json_each(?))"
	args := []any{}
	for title, since := range loadedSince {
		gone += " OR (mi.library_title = ? AND mi.last_seen_at < ?)"
		args = append(args, title, since)
	}
	titlesJSON, err := json.Marshal(usedLibraries)
	if err != nil {
		return fail("Failed to list the libraries", err)
	}
	args = append([]any{string(titlesJSON)}, args...)

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fail("Failed to start transaction", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, "SELECT mi.id FROM MediaItems mi WHERE ("+gone+") AND "+notSavedOrIgnored+";", args...)
	if err != nil {
		return fail("Failed to find the library items that are gone", err)
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fail("Failed to read a library item", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fail("Failed to read the library items that are gone", err)
	}
	for _, id := range ids {
		if err := deleteMediaItemRow(ctx, tx, id); err != nil {
			return fail("Failed to remove a library item", err)
		}
	}

	if _, err := tx.ExecContext(ctx, "UPDATE MediaItems AS mi SET on_server = 0 WHERE on_server = 1 AND ("+gone+");", args...); err != nil {
		return fail("Failed to mark the saved items that are gone", err)
	}
	if err := tx.Commit(); err != nil {
		return fail("Failed to commit transaction", err)
	}
	return logging.LogErrorInfo{}
}

// GetLibraryItemsOnServer returns the main media server's items that are on the server
func (s *SQliteDB) GetLibraryItemsOnServer(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	items := []models.MediaItem{}
	fail := func(message string, err error) ([]models.MediaItem, logging.LogErrorInfo) {
		return items, dbError(ctx, "Getting the library items from the database", message, err)
	}

	rows, err := s.conn.QueryContext(ctx, `
		SELECT
			mi.tmdb_id, mi.library_title, mi.edition, mi.rating_key, mi.type, mi.title, mi.year,
			mi.added_at, mi.updated_at, mi.released_at, mi.latest_episode_added_at, mi.content_rating, mi.summary, mi.guids,
			mv.path, mv.size, mv.duration
		FROM MediaItems mi
		LEFT JOIN Movies mv ON mv.media_item_id = mi.id
		WHERE mi.on_server = 1
		ORDER BY mi.library_title, mi.id;
	`)
	if err != nil {
		return fail("Failed to get the library items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item models.MediaItem
		var guids string
		var path sql.NullString
		var size, duration sql.NullInt64
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year,
			&item.AddedAt, &item.UpdatedAt, &item.ReleasedAt, &item.LatestEpisodeAddedAt, &item.ContentRating, &item.Summary, &guids,
			&path, &size, &duration); err != nil {
			return fail("Failed to read a library item", err)
		}
		_ = json.Unmarshal([]byte(guids), &item.Guids)
		if path.Valid {
			item.Movie = &models.MediaItemMovie{File: models.MediaItemFile{Path: path.String, Size: size.Int64, Duration: duration.Int64}}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fail("Failed to read the library items", err)
	}
	return items, logging.LogErrorInfo{}
}
