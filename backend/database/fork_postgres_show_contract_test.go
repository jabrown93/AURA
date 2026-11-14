package database

import (
	"aura/models"
	"context"
	"testing"
	"time"
)

// TestShowStorageContract checks nested episodes, partial refreshes, and last-set ownership on both engines.
func TestShowStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			ctx := context.Background()
			var db DB
			if engine == "sqlite" {
				db = openTestMediaDB(t)
			} else {
				pg := newPostgresTestDB(t)
				if _, info := pg.Init(ctx); info.Message != "" {
					t.Fatal(info)
				}
				db = pg
			}
			date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			season, episode := 1, 2
			item := models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: "show-1", LibraryTitle: "Shows", RatingKey: "100", Type: "show", Title: "Show", Year: 2026, Series: &models.MediaItemSeries{SeasonCount: 1, EpisodeCount: 1, Location: "/shows/show", Seasons: []models.MediaItemSeason{{RatingKey: "101", SeasonNumber: season, Episodes: []models.MediaItemEpisode{{RatingKey: "102", SeasonNumber: season, EpisodeNumber: episode, Title: "Episode", File: models.MediaItemFile{Path: "/shows/show/episode.mkv", Size: 123, Duration: 456}}}}}}}, PosterSets: []models.DBPosterSetDetail{
				{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "first", Type: "show", Title: "First", DateCreated: date}, Images: []models.ImageFile{{ID: "card", Type: "titlecard", Modified: date, SeasonNumber: &season, EpisodeNumber: &episode}}}, LastDownloaded: date, SelectedTypes: models.SelectedTypes{Poster: true, Titlecard: true}},
				{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "second", Type: "show", Title: "Second", DateCreated: date}}, LastDownloaded: date, SelectedTypes: models.SelectedTypes{Poster: true}},
			}}
			if info := db.UpsertSavedItem(ctx, item); info.Message != "" {
				t.Fatal(info)
			}
			check := func() {
				t.Helper()
				out, info := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
				if info.Message != "" || len(out.Items) != 1 {
					t.Fatalf("show: %+v %+v", out, info)
				}
				saved := out.Items[0]
				if saved.MediaItem.Series == nil || len(saved.MediaItem.Series.Seasons) != 1 || len(saved.MediaItem.Series.Seasons[0].Episodes) != 1 {
					t.Fatalf("lost hierarchy: %+v", saved.MediaItem.Series)
				}
				got := saved.MediaItem.Series.Seasons[0].Episodes[0]
				if got.EpisodeNumber != episode || got.File.Path != "/shows/show/episode.mkv" || got.File.Size != 123 {
					t.Fatalf("lost episode: %+v", got)
				}
				for _, set := range saved.PosterSets {
					if set.ID == "first" && (set.SelectedTypes.Poster || !set.SelectedTypes.Titlecard || len(set.Images) != 1) {
						t.Fatalf("ownership: %+v", set)
					}
					if set.ID == "second" && !set.SelectedTypes.Poster {
						t.Fatal("last set lost poster ownership")
					}
				}
			}
			check()
			item.MediaItem.Series.Seasons = nil
			if info := db.UpsertSavedItem(ctx, item); info.Message != "" {
				t.Fatal(info)
			}
			check()
		})
	}
}
