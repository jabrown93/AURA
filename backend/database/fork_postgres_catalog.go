package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"time"
)

// SaveCollections atomically replaces the main server's collection catalog.
func (p *PostgresDB) SaveCollections(ctx context.Context, collections []models.CollectionItem) logging.LogErrorInfo {
	tx, err := p.postgresSavedTx(ctx)
	if err != nil {
		return postgresError("Failed to begin collection save", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM CollectionItems"); err != nil {
		return postgresError("Failed to clear collection items", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM Collections"); err != nil {
		return postgresError("Failed to clear collections", err)
	}
	for _, collection := range collections {
		if collection.RatingKey == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO Collections (rating_key,library_title,collection_index,tmdb_id,title,summary,child_count)
VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (rating_key) DO NOTHING`, collection.RatingKey, collection.LibraryTitle, collection.Index, collection.TMDB_ID, collection.Title, collection.Summary, collection.ChildCount); err != nil {
			return postgresError("Failed to save collection", err)
		}
		for _, item := range collection.MediaItems {
			if item.RatingKey == "" {
				continue
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO CollectionItems (collection_rating_key,rating_key,tmdb_id,library_title,title,year)
VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (collection_rating_key,rating_key) DO NOTHING`, collection.RatingKey, item.RatingKey, item.TMDB_ID, item.LibraryTitle, item.Title, item.Year); err != nil {
				return postgresError("Failed to save collection item", err)
			}
		}
	}
	return postgresError("Failed to commit collection save", tx.Commit())
}

// GetCollections reads catalog and membership in one consistent statement.
func (p *PostgresDB) GetCollections(ctx context.Context) ([]models.CollectionItem, logging.LogErrorInfo) {
	collections := []models.CollectionItem{}
	rows, err := p.conn.QueryContext(ctx, `SELECT c.rating_key,c.library_title,c.collection_index,c.tmdb_id,c.title,c.summary,c.child_count,
(SELECT jsonb_agg(jsonb_build_object('rating_key',ci.rating_key,'tmdb_id',ci.tmdb_id,'library_title',ci.library_title,'title',ci.title,'year',ci.year,'type','movie') ORDER BY ci.year,ci.rating_key)
FROM CollectionItems ci WHERE ci.collection_rating_key=c.rating_key)
FROM Collections c ORDER BY c.library_title,c.title,c.rating_key`)
	if err != nil {
		return collections, postgresError("Failed to query collections", err)
	}
	defer rows.Close()
	for rows.Next() {
		var collection models.CollectionItem
		var itemsJSON []byte
		if err := rows.Scan(&collection.RatingKey, &collection.LibraryTitle, &collection.Index, &collection.TMDB_ID, &collection.Title, &collection.Summary, &collection.ChildCount, &itemsJSON); err != nil {
			return collections, postgresError("Failed to scan collection", err)
		}
		if len(itemsJSON) > 0 {
			if err := json.Unmarshal(itemsJSON, &collection.MediaItems); err != nil {
				return collections, postgresError("Failed to decode collection items", err)
			}
		}
		collections = append(collections, collection)
	}
	return collections, postgresError("Failed to read collections", rows.Err())
}

// UpsertSiloItemIDs preserves source timestamps so stale catalog entries can be refreshed.
func (p *PostgresDB) UpsertSiloItemIDs(ctx context.Context, items []SiloItemIDs) logging.LogErrorInfo {
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return postgresError("Failed to begin Silo catalog save", err)
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err = tx.ExecContext(ctx, `INSERT INTO SiloItemIDs (content_id,tmdb_id,imdb_id,tvdb_id,first_air_date,season_count,episode_count,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (content_id) DO UPDATE SET tmdb_id=EXCLUDED.tmdb_id,imdb_id=EXCLUDED.imdb_id,tvdb_id=EXCLUDED.tvdb_id,
first_air_date=EXCLUDED.first_air_date,season_count=EXCLUDED.season_count,episode_count=EXCLUDED.episode_count,updated_at=EXCLUDED.updated_at`,
			item.ContentID, item.TMDB_ID, item.IMDB_ID, item.TVDB_ID, item.FirstAirDate, item.SeasonCount, item.EpisodeCount, item.UpdatedAt); err != nil {
			return postgresError("Failed to save Silo catalog item", err)
		}
	}
	return postgresError("Failed to commit Silo catalog save", tx.Commit())
}

// GetAllSiloItemIDs restores provider IDs and counts without another Silo scan.
func (p *PostgresDB) GetAllSiloItemIDs(ctx context.Context) ([]SiloItemIDs, logging.LogErrorInfo) {
	items := []SiloItemIDs{}
	rows, err := p.conn.QueryContext(ctx, "SELECT content_id,tmdb_id,imdb_id,tvdb_id,first_air_date,season_count,episode_count,updated_at FROM SiloItemIDs ORDER BY content_id")
	if err != nil {
		return items, postgresError("Failed to query Silo catalog", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item SiloItemIDs
		if err := rows.Scan(&item.ContentID, &item.TMDB_ID, &item.IMDB_ID, &item.TVDB_ID, &item.FirstAirDate, &item.SeasonCount, &item.EpisodeCount, &item.UpdatedAt); err != nil {
			return items, postgresError("Failed to scan Silo catalog item", err)
		}
		items = append(items, item)
	}
	return items, postgresError("Failed to read Silo catalog", rows.Err())
}

// InsertDownloadEvent stores immutable statistics events with native timestamps and JSON counts.
func (p *PostgresDB) InsertDownloadEvent(ctx context.Context, event models.DownloadEvent) logging.LogErrorInfo {
	if event.ImageTypes == nil {
		event.ImageTypes = map[string]int{}
	}
	imageTypes, err := json.Marshal(event.ImageTypes)
	if err != nil {
		return postgresError("Failed to encode event image types", err)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	_, err = p.conn.ExecContext(ctx, `INSERT INTO DownloadEvents
(tmdb_id,library_title,edition,media_item_title,media_item_type,set_id,set_title,set_type,set_creator,source,kind,images_succeeded,images_failed,image_types,bytes,estimated,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		event.TMDB_ID, event.LibraryTitle, event.Edition, event.MediaItemTitle, event.MediaItemType, event.SetID, event.SetTitle, event.SetType, event.SetCreator, event.Source, event.Kind,
		event.ImagesSucceeded, event.ImagesFailed, string(imageTypes), event.Bytes, boolToInt(event.Estimated), event.CreatedAt)
	return postgresError("Failed to save download event", err)
}
