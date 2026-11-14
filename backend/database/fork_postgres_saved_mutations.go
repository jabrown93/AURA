package database

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// CheckIfMediaItemExists keeps ignore choices separate from saved-set ownership.
func (p *PostgresDB) CheckIfMediaItemExists(ctx context.Context, tmdbID, library, edition string) (bool, string, []models.DBSavedSet, logging.LogErrorInfo) {
	sets := []models.DBSavedSet{}
	var mode string
	err := p.conn.QueryRowContext(ctx, "SELECT mode FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", tmdbID, library, edition).Scan(&mode)
	if err == nil {
		return true, strings.TrimSpace(mode), sets, logging.LogErrorInfo{}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, "", sets, postgresError("Failed to read ignored status", err)
	}
	rows, err := p.conn.QueryContext(ctx, `SELECT ps.set_id,ps."user",si.poster_selected=1,si.backdrop_selected=1,si.season_poster_selected=1,si.special_season_poster_selected=1,si.titlecard_selected=1
FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id WHERE si.tmdb_id=$1 AND si.library_title=$2 AND si.edition=$3 ORDER BY ps.id`, tmdbID, library, edition)
	if err != nil {
		return false, "", sets, postgresError("Failed to read saved-set flags", err)
	}
	defer rows.Close()
	for rows.Next() {
		var set models.DBSavedSet
		selected := &set.SelectedTypes
		if err := rows.Scan(&set.ID, &set.UserCreated, &selected.Poster, &selected.Backdrop, &selected.SeasonPoster, &selected.SpecialSeasonPoster, &selected.Titlecard); err != nil {
			return false, "", sets, postgresError("Failed to scan saved-set flags", err)
		}
		sets = append(sets, set)
	}
	return false, "", sets, postgresError("Failed to read saved-set flags", rows.Err())
}

// IgnoreMediaItem validates modes before storing the upstream comma-separated set IDs as JSON.
func (p *PostgresDB) IgnoreMediaItem(ctx context.Context, tmdbID, library, edition, mode, currentSets string) logging.LogErrorInfo {
	tmdbID, library, edition = strings.TrimSpace(tmdbID), strings.TrimSpace(library), strings.TrimSpace(edition)
	mode = strings.ToLower(strings.TrimSpace(mode))
	if tmdbID == "" || library == "" {
		return logging.LogErrorInfo{Message: "tmdb_id and library_title are required"}
	}
	if mode != "always" && mode != "until-set-available" && mode != "until-new-set-available" {
		return logging.LogErrorInfo{Message: "Invalid ignore mode"}
	}
	if mode == "until-new-set-available" && currentSets == "" {
		return logging.LogErrorInfo{Message: "current_sets is required for 'until-new-set-available' mode"}
	}
	sets := []string{}
	if currentSets != "" {
		sets = strings.Split(currentSets, ",")
	}
	data, err := json.Marshal(sets)
	if err != nil {
		return postgresError("Failed to encode ignored sets", err)
	}
	_, err = p.conn.ExecContext(ctx, `INSERT INTO IgnoredItems (tmdb_id,library_title,edition,mode,current_sets) VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (tmdb_id,library_title,edition) DO UPDATE SET mode=EXCLUDED.mode,current_sets=EXCLUDED.current_sets`, tmdbID, library, edition, mode, string(data))
	return postgresError("Failed to ignore media item", err)
}

// AutoIgnoreMediaItem never replaces a user's existing ignore choice.
func (p *PostgresDB) AutoIgnoreMediaItem(ctx context.Context, tmdbID, library, edition string) logging.LogErrorInfo {
	tmdbID, library, edition = strings.TrimSpace(tmdbID), strings.TrimSpace(library), strings.TrimSpace(edition)
	if tmdbID == "" || library == "" {
		return logging.LogErrorInfo{Message: "tmdb_id and library_title are required"}
	}
	_, err := p.conn.ExecContext(ctx, `INSERT INTO IgnoredItems (tmdb_id,library_title,edition,mode,current_sets)
VALUES ($1,$2,$3,'until-set-available','[]') ON CONFLICT (tmdb_id,library_title,edition) DO NOTHING`, tmdbID, library, edition)
	return postgresError("Failed to auto-ignore media item", err)
}

// StopIgnoringMediaItem preserves the existing API's whitespace normalization.
func (p *PostgresDB) StopIgnoringMediaItem(ctx context.Context, tmdbID, library, edition string) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "DELETE FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", strings.TrimSpace(tmdbID), strings.TrimSpace(library), strings.TrimSpace(edition))
	return postgresError("Failed to stop ignoring media item", err)
}

// GetTempIgnoredItems only returns items still present in the media-server cache.
func (p *PostgresDB) GetTempIgnoredItems(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	items := []models.MediaItem{}
	rows, err := p.conn.QueryContext(ctx, "SELECT tmdb_id,library_title,edition,mode,current_sets FROM IgnoredItems WHERE mode IN ('until-set-available','until-new-set-available')")
	if err != nil {
		return nil, postgresError("Failed to query temporarily ignored items", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tmdbID, library, edition, mode string
		var data []byte
		if err := rows.Scan(&tmdbID, &library, &edition, &mode, &data); err != nil {
			return nil, postgresError("Failed to scan ignored item", err)
		}
		var sets []string
		if err := json.Unmarshal(data, &sets); err != nil {
			return nil, postgresError("Failed to decode ignored sets", err)
		}
		if item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(library, tmdbID, edition); found {
			item.IgnoredMode, item.IgnoredSets = mode, sets
			items = append(items, *item)
		}
	}
	return items, postgresError("Failed to read ignored items", rows.Err())
}

// SaveDownloadedImage records progress only for an already-saved item/set link.
func (p *PostgresDB) SaveDownloadedImage(ctx context.Context, item models.MediaItem, setID string, image models.ImageFile) (bool, logging.LogErrorInfo) {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return false, postgresError("Failed to begin downloaded-image save", err)
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT si.poster_set_id FROM SavedItems si JOIN PosterSets ps ON ps.id=si.poster_set_id
WHERE si.tmdb_id=$1 AND si.library_title=$2 AND si.edition=$3 AND ps.set_id=$4`, item.TMDB_ID, item.LibraryTitle, item.Edition, setID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, logging.LogErrorInfo{}
	}
	if err != nil {
		return false, postgresError("Failed to find downloaded image's set", err)
	}
	if err = postgresUpsertImage(ctx, tx, id, item.TMDB_ID, image); err != nil {
		return false, postgresError("Failed to save downloaded image", err)
	}
	if _, err = tx.ExecContext(ctx, "UPDATE SavedItems SET last_downloaded=$1 WHERE tmdb_id=$2 AND library_title=$3 AND edition=$4 AND poster_set_id=$5", time.Now(), item.TMDB_ID, item.LibraryTitle, item.Edition, id); err != nil {
		return false, postgresError("Failed to update download timestamp", err)
	}
	if err = tx.Commit(); err != nil {
		return false, postgresError("Failed to commit downloaded image", err)
	}
	return true, logging.LogErrorInfo{}
}

// DeletePosterSetForMediaItem removes one edition's link without deleting shared sets.
func (p *PostgresDB) DeletePosterSetForMediaItem(ctx context.Context, tmdbID, library, edition, setID string) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin saved-set deletion", err)
	}
	defer tx.Rollback()
	if err = postgresDeleteSetLink(ctx, tx, tmdbID, library, edition, setID); err != nil {
		return postgresError("Failed to delete saved-set link", err)
	}
	return postgresError("Failed to commit saved-set deletion", tx.Commit())
}

// DeleteAllPosterSetsForMediaItem deletes item-scoped images before unlinking its sets.
func (p *PostgresDB) DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, library, edition string) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin saved-set deletion", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM ImageFiles WHERE item_tmdb_id=$1 AND poster_set_id IN
(SELECT poster_set_id FROM SavedItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3)`, tmdbID, library, edition); err != nil {
		return postgresError("Failed to delete item-scoped images", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM SavedItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", tmdbID, library, edition); err != nil {
		return postgresError("Failed to delete saved-set links", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM PosterSets WHERE NOT EXISTS (SELECT 1 FROM SavedItems WHERE poster_set_id=PosterSets.id)"); err != nil {
		return postgresError("Failed to delete orphan sets", err)
	}
	return postgresError("Failed to commit saved-set deletion", tx.Commit())
}

// DeleteMediaItemAndIgnoredStatus cascades the media hierarchy and saved links atomically.
func (p *PostgresDB) DeleteMediaItemAndIgnoredStatus(ctx context.Context, tmdbID, library, edition string) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin media-item deletion", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", tmdbID, library, edition); err != nil {
		return postgresError("Failed to delete ignored status", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM MediaItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", tmdbID, library, edition); err != nil {
		return postgresError("Failed to delete media item", err)
	}
	return postgresError("Failed to commit media-item deletion", tx.Commit())
}

// UpdateMediaItem refreshes identity details without recreating saved media or seasons.
func (p *PostgresDB) UpdateMediaItem(ctx context.Context, item models.MediaItem) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "UPDATE MediaItems SET rating_key=$1,type=$2,title=$3,year=$4 WHERE tmdb_id=$5 AND library_title=$6 AND edition=$7", item.RatingKey, item.Type, item.Title, item.Year, item.TMDB_ID, item.LibraryTitle, item.Edition)
	return postgresError("Failed to update media-item details", err)
}

// UpdateMediaItemOnServer retains the upstream integer storage contract.
func (p *PostgresDB) UpdateMediaItemOnServer(ctx context.Context, tmdbID, library, edition string, onServer bool) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "UPDATE MediaItems SET on_server=$1 WHERE tmdb_id=$2 AND library_title=$3 AND edition=$4", boolToInt(onServer), tmdbID, library, edition)
	return postgresError("Failed to update media-item availability", err)
}
