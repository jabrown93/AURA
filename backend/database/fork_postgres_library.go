package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// SaveLibraryItems records listing metadata without replacing saved-set or episode details.
func (p *PostgresDB) SaveLibraryItems(ctx context.Context, items []models.MediaItem, seenAt int64) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin library save", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO MediaItems
(tmdb_id,library_title,edition,rating_key,type,title,year,on_server,added_at,updated_at,released_at,latest_episode_added_at,content_rating,summary,guids,first_seen_at,last_seen_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,$11,$12,$13,$14,$15,$15)
ON CONFLICT (tmdb_id,library_title,edition) DO UPDATE SET rating_key=EXCLUDED.rating_key,type=EXCLUDED.type,title=EXCLUDED.title,year=EXCLUDED.year,on_server=1,
added_at=EXCLUDED.added_at,updated_at=EXCLUDED.updated_at,released_at=EXCLUDED.released_at,latest_episode_added_at=EXCLUDED.latest_episode_added_at,content_rating=EXCLUDED.content_rating,summary=EXCLUDED.summary,guids=EXCLUDED.guids,
first_seen_at=CASE WHEN MediaItems.first_seen_at=0 THEN EXCLUDED.first_seen_at ELSE MediaItems.first_seen_at END,last_seen_at=EXCLUDED.last_seen_at`)
	if err != nil {
		return postgresError("Failed to prepare library save", err)
	}
	defer stmt.Close()
	for _, item := range items {
		if item.TMDB_ID == "" || (item.Type != "movie" && item.Type != "show") {
			continue
		}
		guids, err := json.Marshal(item.Guids)
		if err != nil {
			return postgresError("Failed to encode library GUIDs", err)
		}
		if item.Guids == nil {
			guids = []byte("[]")
		}
		if _, err := stmt.ExecContext(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, item.RatingKey, item.Type, item.Title, item.Year,
			item.AddedAt, item.UpdatedAt, item.ReleasedAt, item.LatestEpisodeAddedAt, item.ContentRating, item.Summary, string(guids), seenAt); err != nil {
			return postgresError("Failed to save library item", err)
		}
	}
	return postgresError("Failed to commit library save", tx.Commit())
}

// FinishLibraryLoad retains saved and ignored rows when their media-server items disappear.
func (p *PostgresDB) FinishLibraryLoad(ctx context.Context, usedLibraries []string, loadedSince map[string]int64) logging.LogErrorInfo {
	if usedLibraries == nil {
		usedLibraries = []string{}
	}
	gone := "NOT (mi.library_title=ANY($1::text[]))"
	args := []any{usedLibraries}
	for library, since := range loadedSince {
		gone += fmt.Sprintf(" OR (mi.library_title=$%d AND mi.last_seen_at<$%d)", len(args)+1, len(args)+2)
		args = append(args, library, since)
	}
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin library cleanup", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM MediaItems mi WHERE ("+gone+") AND "+notSavedOrIgnored, args...); err != nil {
		return postgresError("Failed to remove missing library items", err)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE MediaItems mi SET on_server=0 WHERE on_server=1 AND ("+gone+")", args...); err != nil {
		return postgresError("Failed to mark missing saved items", err)
	}
	return postgresError("Failed to commit library cleanup", tx.Commit())
}

// GetLibraryItemsOnServer keeps upstream's lightweight listing shape, without loading episode trees.
func (p *PostgresDB) GetLibraryItemsOnServer(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	items := []models.MediaItem{}
	rows, err := p.conn.QueryContext(ctx, `SELECT mi.tmdb_id,mi.library_title,mi.edition,mi.rating_key,mi.type,mi.title,mi.year,
mi.added_at,mi.updated_at,mi.released_at,mi.latest_episode_added_at,mi.content_rating,mi.summary,mi.guids,mv.path,mv.size,mv.duration
FROM MediaItems mi LEFT JOIN Movies mv ON mv.media_item_id=mi.id WHERE mi.on_server=1 ORDER BY mi.library_title,mi.id`)
	if err != nil {
		return items, postgresError("Failed to list library items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item models.MediaItem
		var guids []byte
		var path sql.NullString
		var size, duration sql.NullInt64
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year,
			&item.AddedAt, &item.UpdatedAt, &item.ReleasedAt, &item.LatestEpisodeAddedAt, &item.ContentRating, &item.Summary, &guids, &path, &size, &duration); err != nil {
			return items, postgresError("Failed to scan library item", err)
		}
		if err := json.Unmarshal(guids, &item.Guids); err != nil {
			return items, postgresError("Failed to decode library GUIDs", err)
		}
		if path.Valid {
			item.Movie = &models.MediaItemMovie{File: models.MediaItemFile{Path: path.String, Size: size.Int64, Duration: duration.Int64}}
		}
		items = append(items, item)
	}
	return items, postgresError("Failed to read library items", rows.Err())
}

// GetAllMediaItems returns saved or ignored identities, not every cached library item.
func (p *PostgresDB) GetAllMediaItems(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	items := []models.MediaItem{}
	rows, err := p.conn.QueryContext(ctx, "SELECT tmdb_id,library_title,edition,rating_key,type,title,year FROM MediaItems mi WHERE NOT ("+notSavedOrIgnored+") ORDER BY id")
	if err != nil {
		return items, postgresError("Failed to query saved media identities", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item models.MediaItem
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year); err != nil {
			return items, postgresError("Failed to scan media identity", err)
		}
		items = append(items, item)
	}
	return items, postgresError("Failed to read media identities", rows.Err())
}

// GetAllMediaItemsWithFlags uses EXISTS rather than nonstandard SQLite GROUP BY projections.
func (p *PostgresDB) GetAllMediaItemsWithFlags(ctx context.Context) ([]MediaItemWithFlags, logging.LogErrorInfo) {
	items := []MediaItemWithFlags{}
	rows, err := p.conn.QueryContext(ctx, `SELECT mi.tmdb_id,mi.library_title,mi.edition,mi.rating_key,mi.type,mi.title,mi.year,
EXISTS (SELECT 1 FROM SavedItems si WHERE si.tmdb_id=mi.tmdb_id AND si.library_title=mi.library_title AND si.edition=mi.edition),
EXISTS (SELECT 1 FROM IgnoredItems ii WHERE ii.tmdb_id=mi.tmdb_id AND ii.library_title=mi.library_title AND ii.edition=mi.edition)
FROM MediaItems mi ORDER BY mi.id`)
	if err != nil {
		return items, postgresError("Failed to query media flags", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item MediaItemWithFlags
		if err := rows.Scan(&item.TMDB_ID, &item.LibraryTitle, &item.Edition, &item.RatingKey, &item.Type, &item.Title, &item.Year, &item.HasSavedSet, &item.IsIgnored); err != nil {
			return items, postgresError("Failed to scan media flags", err)
		}
		items = append(items, item)
	}
	return items, postgresError("Failed to read media flags", rows.Err())
}

// GetLibraryDBStates includes ignored keys even when no library listing is currently available.
func (p *PostgresDB) GetLibraryDBStates(ctx context.Context, library string) (map[string]MediaItemDBState, logging.LogErrorInfo) {
	states := map[string]MediaItemDBState{}
	rows, err := p.conn.QueryContext(ctx, `WITH keys AS (
SELECT tmdb_id,edition FROM IgnoredItems WHERE library_title=$1 UNION SELECT tmdb_id,edition FROM SavedItems WHERE library_title=$1)
SELECT k.tmdb_id,k.edition,COALESCE(ii.mode,''),COALESCE((SELECT jsonb_agg(jsonb_build_object('id',ps.set_id,'user_created',ps."user",'selected_types',
jsonb_build_object('poster',si.poster_selected=1,'backdrop',si.backdrop_selected=1,'season_poster',si.season_poster_selected=1,'special_season_poster',si.special_season_poster_selected=1,'titlecard',si.titlecard_selected=1)) ORDER BY ps.id)
FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id WHERE si.tmdb_id=k.tmdb_id AND si.edition=k.edition AND si.library_title=$1),'[]'::jsonb)
FROM keys k LEFT JOIN IgnoredItems ii ON ii.tmdb_id=k.tmdb_id AND ii.edition=k.edition AND ii.library_title=$1`, library)
	if err != nil {
		return states, postgresError("Failed to query library database states", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tmdbID, edition, mode string
		var setsJSON []byte
		if err := rows.Scan(&tmdbID, &edition, &mode, &setsJSON); err != nil {
			return states, postgresError("Failed to scan library database state", err)
		}
		state := MediaItemDBState{IgnoredMode: strings.TrimSpace(mode)}
		state.Ignored = state.IgnoredMode != ""
		if !state.Ignored {
			if err := json.Unmarshal(setsJSON, &state.Sets); err != nil {
				return states, postgresError("Failed to decode library saved sets", err)
			}
		}
		states[dbStateKey(tmdbID, edition)] = state
	}
	return states, postgresError("Failed to read library database states", rows.Err())
}

// GetSavedSetCreators counts shared sets once, regardless of how many items use them.
func (p *PostgresDB) GetSavedSetCreators(ctx context.Context) ([]CreatorSetCount, logging.LogErrorInfo) {
	creators := []CreatorSetCount{}
	rows, err := p.conn.QueryContext(ctx, `SELECT ps."user",count(DISTINCT ps.id) AS sets FROM PosterSets ps JOIN SavedItems si ON si.poster_set_id=ps.id
WHERE ps."user"<>'' GROUP BY ps."user" ORDER BY sets DESC,lower(ps."user"),ps."user"`)
	if err != nil {
		return creators, postgresError("Failed to query saved-set creators", err)
	}
	defer rows.Close()
	for rows.Next() {
		var creator CreatorSetCount
		if err := rows.Scan(&creator.Username, &creator.Sets); err != nil {
			return creators, postgresError("Failed to scan saved-set creator", err)
		}
		creators = append(creators, creator)
	}
	return creators, postgresError("Failed to read saved-set creators", rows.Err())
}

// GetAllUniqueUsers retains upstream's inclusion of unnamed creators.
func (p *PostgresDB) GetAllUniqueUsers(ctx context.Context) ([]string, logging.LogErrorInfo) {
	users := []string{}
	rows, err := p.conn.QueryContext(ctx, `SELECT DISTINCT "user" FROM PosterSets ORDER BY "user"`)
	if err != nil {
		return users, postgresError("Failed to query saved-set users", err)
	}
	defer rows.Close()
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return users, postgresError("Failed to scan saved-set user", err)
		}
		users = append(users, user)
	}
	return users, postgresError("Failed to read saved-set users", rows.Err())
}
