package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// postgresSavedTx serializes saved-set ownership and orphan cleanup, as SQLite does.
func (p *PostgresDB) postgresSavedTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1096110658)"); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// UpsertSavedItem preserves upstream's all-or-nothing saved-set and selected-type ownership rules.
func (p *PostgresDB) UpsertSavedItem(ctx context.Context, item models.DBSavedItem) logging.LogErrorInfo {
	if (item.MediaItem.Type == "movie" && item.MediaItem.Movie == nil) || (item.MediaItem.Type == "show" && item.MediaItem.Series == nil) {
		return logging.LogErrorInfo{Message: "Saved media item is missing its type details"}
	}
	item = item.WithSetImageTypes()
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin saved-item update", err)
	}
	defer tx.Rollback()
	media := item.MediaItem
	if _, err = tx.ExecContext(ctx, "DELETE FROM IgnoredItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3", media.TMDB_ID, media.LibraryTitle, media.Edition); err != nil {
		return postgresError("Failed to clear ignored status", err)
	}
	if _, err = postgresUpsertMedia(ctx, tx, media); err != nil {
		return postgresError("Failed to save media item", err)
	}
	owners := map[string]string{}
	for _, set := range item.PosterSets {
		if set.ToDelete {
			continue
		}
		if err := postgresDeleteSetLink(ctx, tx, media.TMDB_ID, media.LibraryTitle, media.Edition, set.ID); err != nil {
			return postgresError("Failed to replace saved-set link", err)
		}
	}
	for _, set := range item.PosterSets {
		if set.ToDelete {
			continue
		}
		set.DateUpdated = time.Now().UTC()
		var setID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO PosterSets (set_id,type,title,"user",date_created,date_updated)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (set_id) DO UPDATE SET
 type=EXCLUDED.type,title=EXCLUDED.title,"user"=EXCLUDED."user",date_updated=EXCLUDED.date_updated RETURNING id`,
			set.ID, set.Type, set.Title, set.UserCreated, set.DateCreated, set.DateUpdated).Scan(&setID)
		if err != nil {
			return postgresError("Failed to save poster set", err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO SavedItems
(tmdb_id,library_title,edition,poster_set_id,poster_selected,backdrop_selected,season_poster_selected,special_season_poster_selected,titlecard_selected,autodownload,auto_add_new_collection_items,no_auto_upgrade,force_preload_missing,last_downloaded)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			media.TMDB_ID, media.LibraryTitle, media.Edition, setID,
			boolToInt(set.SelectedTypes.Poster), boolToInt(set.SelectedTypes.Backdrop), boolToInt(set.SelectedTypes.SeasonPoster), boolToInt(set.SelectedTypes.SpecialSeasonPoster), boolToInt(set.SelectedTypes.Titlecard),
			boolToInt(set.AutoDownload), boolToInt(set.AutoAddNewCollectionItems), boolToInt(set.NoAutoUpgrade), boolToInt(set.ForcePreloadMissing), lastDownloaded(set.LastDownloaded))
		if err != nil {
			return postgresError("Failed to save item-set link", err)
		}
		for _, image := range set.Images {
			if err = postgresUpsertImage(ctx, tx, setID, media.TMDB_ID, image); err != nil {
				return postgresError("Failed to save image", err)
			}
		}
		for column, selected := range map[string]bool{
			"poster_selected": set.SelectedTypes.Poster, "backdrop_selected": set.SelectedTypes.Backdrop,
			"season_poster_selected": set.SelectedTypes.SeasonPoster, "special_season_poster_selected": set.SelectedTypes.SpecialSeasonPoster, "titlecard_selected": set.SelectedTypes.Titlecard,
		} {
			if selected {
				owners[column] = set.ID
			}
		}
	}
	for column, owner := range owners {
		// Column names come only from the literal whitelist above, never request input.
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`UPDATE SavedItems SET %s=0 WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3
AND poster_set_id<>(SELECT id FROM PosterSets WHERE set_id=$4)`, column), media.TMDB_ID, media.LibraryTitle, media.Edition, owner)
		if err != nil {
			return postgresError("Failed to reconcile selected image types", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM SavedItems WHERE tmdb_id=$1 AND library_title=$2 AND edition=$3
AND poster_selected=0 AND backdrop_selected=0 AND season_poster_selected=0 AND special_season_poster_selected=0 AND titlecard_selected=0`, media.TMDB_ID, media.LibraryTitle, media.Edition); err != nil {
		return postgresError("Failed to remove unselected sets", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM PosterSets WHERE NOT EXISTS (SELECT 1 FROM SavedItems WHERE poster_set_id=PosterSets.id)"); err != nil {
		return postgresError("Failed to remove orphan poster sets", err)
	}
	return postgresError("Failed to commit saved-item update", tx.Commit())
}

// postgresUpsertMedia retains absent season/episode slices instead of deleting unknown details.
func postgresUpsertMedia(ctx context.Context, tx *sql.Tx, item models.MediaItem) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `INSERT INTO MediaItems
(tmdb_id,library_title,edition,rating_key,type,title,year,on_server,first_seen_at,last_seen_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,$8)
ON CONFLICT (tmdb_id,library_title,edition) DO UPDATE SET rating_key=EXCLUDED.rating_key,type=EXCLUDED.type,title=EXCLUDED.title,year=EXCLUDED.year,on_server=1,
first_seen_at=CASE WHEN MediaItems.first_seen_at=0 THEN EXCLUDED.first_seen_at ELSE MediaItems.first_seen_at END,last_seen_at=EXCLUDED.last_seen_at RETURNING id`,
		item.TMDB_ID, item.LibraryTitle, item.Edition, item.RatingKey, item.Type, item.Title, item.Year, time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	switch item.Type {
	case "movie":
		if item.Movie == nil {
			return id, nil
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO Movies (media_item_id,path,size,duration) VALUES ($1,$2,$3,$4)
ON CONFLICT (media_item_id) DO UPDATE SET path=EXCLUDED.path,size=EXCLUDED.size,duration=EXCLUDED.duration`, id, item.Movie.File.Path, item.Movie.File.Size, item.Movie.File.Duration)
	case "show":
		if item.Series == nil {
			return id, nil
		}
		var seriesID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO Series (media_item_id,season_count,episode_count,location) VALUES ($1,$2,$3,$4)
ON CONFLICT (media_item_id) DO UPDATE SET season_count=EXCLUDED.season_count,episode_count=EXCLUDED.episode_count,location=EXCLUDED.location RETURNING id`,
			id, item.Series.SeasonCount, item.Series.EpisodeCount, item.Series.Location).Scan(&seriesID)
		if err == nil {
			err = postgresReconcileSeasons(ctx, tx, seriesID, item.Series.Seasons)
		}
	default:
		err = errors.New("unsupported media item type")
	}
	return id, err
}

// postgresReconcileSeasons mirrors upstream reconciliation without translating SQLite SQL at runtime.
func postgresReconcileSeasons(ctx context.Context, tx *sql.Tx, seriesID int64, seasons []models.MediaItemSeason) error {
	if seasons == nil {
		return nil
	}
	seasonNumbers := make([]int, 0, len(seasons))
	for _, season := range seasons {
		seasonNumbers = append(seasonNumbers, season.SeasonNumber)
	}
	if len(seasonNumbers) > 0 {
		if _, err := tx.ExecContext(ctx, "DELETE FROM Seasons WHERE series_id=$1 AND NOT (season_number=ANY($2::bigint[]))", seriesID, seasonNumbers); err != nil {
			return err
		}
	}
	for _, season := range seasons {
		var seasonID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO Seasons (series_id,rating_key,season_number,episode_count) VALUES ($1,$2,$3,$4)
ON CONFLICT (series_id,season_number) DO UPDATE SET rating_key=EXCLUDED.rating_key,episode_count=EXCLUDED.episode_count RETURNING id`,
			seriesID, season.RatingKey, season.SeasonNumber, len(season.Episodes)).Scan(&seasonID); err != nil {
			return err
		}
		if season.Episodes == nil {
			continue
		}
		episodeNumbers := make([]int, 0, len(season.Episodes))
		for _, episode := range season.Episodes {
			episodeNumbers = append(episodeNumbers, episode.EpisodeNumber)
		}
		if len(episodeNumbers) > 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM Episodes WHERE season_id=$1 AND NOT (episode_number=ANY($2::bigint[]))", seasonID, episodeNumbers); err != nil {
				return err
			}
		}
		for _, episode := range season.Episodes {
			_, err := tx.ExecContext(ctx, `INSERT INTO Episodes (season_id,rating_key,episode_number,title,path,size,duration) VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (season_id,episode_number) DO UPDATE SET rating_key=EXCLUDED.rating_key,title=EXCLUDED.title,path=EXCLUDED.path,size=EXCLUDED.size,duration=EXCLUDED.duration`,
				seasonID, episode.RatingKey, episode.EpisodeNumber, episode.Title, episode.File.Path, episode.File.Size, episode.File.Duration)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// postgresUpsertImage scopes shared collection images by the media item's TMDB ID.
func postgresUpsertImage(ctx context.Context, tx *sql.Tx, setID int64, tmdbID string, image models.ImageFile) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO ImageFiles
(poster_set_id,item_tmdb_id,image_id,image_type,image_last_updated,image_season_number,image_episode_number)
VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (poster_set_id,image_id,item_tmdb_id) DO UPDATE SET
image_type=EXCLUDED.image_type,image_last_updated=EXCLUDED.image_last_updated,image_season_number=EXCLUDED.image_season_number,image_episode_number=EXCLUDED.image_episode_number`,
		setID, tmdbID, image.ID, image.Type, image.Modified, image.SeasonNumber, image.EpisodeNumber)
	return err
}

// postgresDeleteSetLink retains shared sets until their final saved-item reference is removed.
func postgresDeleteSetLink(ctx context.Context, tx *sql.Tx, tmdbID, library, edition, setID string) error {
	var id int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM PosterSets WHERE set_id=$1", setID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM ImageFiles WHERE poster_set_id=$1 AND item_tmdb_id=$2", id, tmdbID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM SavedItems WHERE poster_set_id=$1 AND tmdb_id=$2 AND library_title=$3 AND edition=$4", id, tmdbID, library, edition); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM PosterSets WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM SavedItems WHERE poster_set_id=$1)", id)
	return err
}
