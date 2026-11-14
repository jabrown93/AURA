package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

type savedContractDB interface {
	UpsertSavedItem(context.Context, models.DBSavedItem) logging.LogErrorInfo
	GetAllSavedSets(context.Context, models.DBFilter) (PagedSavedItems, logging.LogErrorInfo)
	IgnoreMediaItem(context.Context, string, string, string, string, string) logging.LogErrorInfo
	AutoIgnoreMediaItem(context.Context, string, string, string) logging.LogErrorInfo
	StopIgnoringMediaItem(context.Context, string, string, string) logging.LogErrorInfo
	CheckIfMediaItemExists(context.Context, string, string, string) (bool, string, []models.DBSavedSet, logging.LogErrorInfo)
	DeletePosterSetForMediaItem(context.Context, string, string, string, string) logging.LogErrorInfo
}

// TestSavedStorageContract runs identical nested-JSON and edition fixtures on both engines.
func TestSavedStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db savedContractDB
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
			date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("fixture", 2*3600))
			for _, edition := range []string{"", "Director's Cut"} {
				item := models.DBSavedItem{
					MediaItem:  models.MediaItem{TMDB_ID: "1", LibraryTitle: " Movies", Edition: edition, RatingKey: "100" + edition, Type: "movie", Title: "Fixture", Year: 2026, Movie: &models.MediaItemMovie{}},
					PosterSets: []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set-1", Title: "Set", Type: "movie", UserCreated: "Creator", DateCreated: date}, Images: []models.ImageFile{}}, LastDownloaded: date, SelectedTypes: models.SelectedTypes{Poster: true}, AutoDownload: true, NoAutoUpgrade: true, ForcePreloadMissing: true}},
				}
				if edition != "" {
					item.PosterSets[0].Images = []models.ImageFile{{ID: "image-1", Type: "poster", Modified: date}}
				}
				if info := db.UpsertSavedItem(ctx, item); info.Message != "" {
					t.Fatal(info)
				}
			}
			out, info := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1, LibraryTitles: []string{" Movies"}})
			if info.Message != "" || out.Total != 2 || len(out.Items) != 2 {
				t.Fatalf("saved editions: %+v, %+v", out, info)
			}
			for _, item := range out.Items {
				if len(item.PosterSets) != 1 {
					t.Fatalf("unexpected sets: %+v", item)
				}
				set := item.PosterSets[0]
				if !set.AutoDownload || !set.NoAutoUpgrade || !set.ForcePreloadMissing || !set.SelectedTypes.Poster || set.SelectedTypes.Titlecard || !set.LastDownloaded.Equal(date) || set.Images == nil {
					t.Fatalf("lost booleans, timestamp, or image array: %+v", set)
				}
			}
			out, info = db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: 1, PageNumber: 2, ItemTitle: "fixture"})
			if info.Message != "" || out.Total != 2 || len(out.Items) != 1 || out.Items[0].MediaItem.Edition != "Director's Cut" {
				t.Fatalf("stable case-insensitive pagination: %+v, %+v", out, info)
			}
			invalid := out.Items[0]
			invalid.PosterSets[0].Images = []models.ImageFile{{ID: "invalid", Type: "invalid", Modified: date}}
			if info := db.UpsertSavedItem(ctx, invalid); info.Message == "" {
				t.Fatal("invalid image did not roll back")
			}
			out, info = db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
			if info.Message != "" || len(out.Items) != 2 || len(out.Items[1].PosterSets[0].Images) != 1 || out.Items[1].PosterSets[0].Images[0].ID != "image-1" {
				t.Fatalf("rollback changed saved state: %+v, %+v", out, info)
			}
			if info := db.IgnoreMediaItem(ctx, "1", " Movies", "", "always", ""); info.Message != "" {
				t.Fatal(info)
			}
			if info := db.AutoIgnoreMediaItem(ctx, "1", "Movies", ""); info.Message != "" {
				t.Fatal(info)
			}
			ignored, mode, _, info := db.CheckIfMediaItemExists(ctx, "1", "Movies", "")
			if info.Message != "" || !ignored || mode != "always" {
				t.Fatalf("auto-ignore replaced user choice: %v %q %+v", ignored, mode, info)
			}
			if info := db.StopIgnoringMediaItem(ctx, "1", "Movies", ""); info.Message != "" {
				t.Fatal(info)
			}
			if info := db.DeletePosterSetForMediaItem(ctx, "1", " Movies", "Director's Cut", "set-1"); info.Message != "" {
				t.Fatal(info)
			}
			out, info = db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
			if info.Message != "" || out.Total != 1 || len(out.Items) != 1 || out.Items[0].MediaItem.Edition != "" {
				t.Fatalf("shared-set deletion removed other edition: %+v, %+v", out, info)
			}
		})
	}
}
