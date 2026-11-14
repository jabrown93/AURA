package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

type catalogContractDB interface {
	SaveCollections(context.Context, []models.CollectionItem) logging.LogErrorInfo
	GetCollections(context.Context) ([]models.CollectionItem, logging.LogErrorInfo)
	UpsertSiloItemIDs(context.Context, []SiloItemIDs) logging.LogErrorInfo
	GetAllSiloItemIDs(context.Context) ([]SiloItemIDs, logging.LogErrorInfo)
	InsertDownloadEvent(context.Context, models.DownloadEvent) logging.LogErrorInfo
}

// TestCatalogStorageContract checks collection replacement and persistent Silo metadata.
func TestCatalogStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db catalogContractDB
			if engine == "sqlite" {
				db = openTestMediaDB(t)
			} else {
				pg := newPostgresTestDB(t)
				if _, info := pg.Init(context.Background()); info.Message != "" {
					t.Fatal(info)
				}
				db = pg
			}
			ctx := context.Background()
			collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection", Index: "10", ChildCount: 1, MediaItems: []models.MediaItem{{RatingKey: "movie-1", TMDB_ID: "1", Title: "Movie", LibraryTitle: "Movies", Year: 2026}}}
			if info := db.SaveCollections(ctx, []models.CollectionItem{collection}); info.Message != "" {
				t.Fatal(info)
			}
			out, info := db.GetCollections(ctx)
			if info.Message != "" || len(out) != 1 || len(out[0].MediaItems) != 1 || out[0].MediaItems[0].Type != "movie" || out[0].Index != "10" {
				t.Fatalf("collection: %+v %+v", out, info)
			}
			if info := db.SaveCollections(ctx, nil); info.Message != "" {
				t.Fatal(info)
			}
			if out, info := db.GetCollections(ctx); info.Message != "" || len(out) != 0 {
				t.Fatalf("collections not replaced: %+v %+v", out, info)
			}
			date := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
			item := SiloItemIDs{ContentID: "catalog-1", TMDB_ID: "1", SeasonCount: 2, EpisodeCount: 20, UpdatedAt: date}
			if info := db.UpsertSiloItemIDs(ctx, []SiloItemIDs{item}); info.Message != "" {
				t.Fatal(info)
			}
			item.EpisodeCount = 21
			if info := db.UpsertSiloItemIDs(ctx, []SiloItemIDs{item}); info.Message != "" {
				t.Fatal(info)
			}
			ids, info := db.GetAllSiloItemIDs(ctx)
			if info.Message != "" || len(ids) != 1 || ids[0].EpisodeCount != 21 || !ids[0].UpdatedAt.Equal(date) {
				t.Fatalf("Silo IDs: %+v %+v", ids, info)
			}
			if info := db.InsertDownloadEvent(ctx, models.DownloadEvent{TMDB_ID: "1", LibraryTitle: "Movies", Kind: "download", Source: "download_window", ImagesSucceeded: 1, CreatedAt: date}); info.Message != "" {
				t.Fatal(info)
			}
		})
	}
}
