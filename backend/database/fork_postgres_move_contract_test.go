package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
)

type moveContractDB interface {
	savedContractDB
	MoveMediaItems(context.Context, []MediaItemMove) logging.LogErrorInfo
	ReconcileMediaItemEdition(context.Context, string, string, string, models.MediaItem) logging.LogErrorInfo
}

// TestMoveStorageContract checks overlapping moves do not carry earlier sources onward.
func TestMoveStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db moveContractDB
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
			for _, library := range []string{"A", "B"} {
				set := models.DBPosterSetDetail{SelectedTypes: models.SelectedTypes{Poster: true}}
				set.ID, set.Title, set.Type, set.UserCreated = "set-"+library, "Set", "movie", "Creator"
				item := models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: library, RatingKey: library, Type: "movie", Title: library, Movie: &models.MediaItemMovie{}}, PosterSets: []models.DBPosterSetDetail{set}}
				if info := db.UpsertSavedItem(ctx, item); info.Message != "" {
					t.Fatal(info)
				}
			}
			moves := []MediaItemMove{
				{TMDB_ID: "1", OldLibrary: "A", NewItem: models.MediaItem{LibraryTitle: "B"}},
				{TMDB_ID: "1", OldLibrary: "B", NewItem: models.MediaItem{LibraryTitle: "C"}},
			}
			if info := db.MoveMediaItems(ctx, moves); info.Message != "" {
				t.Fatal(info)
			}
			out, info := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1, SortOption: "library"})
			if info.Message != "" || len(out.Items) != 2 || out.Items[0].MediaItem.LibraryTitle != "B" || out.Items[0].PosterSets[0].ID != "set-A" || out.Items[1].MediaItem.LibraryTitle != "C" || out.Items[1].PosterSets[0].ID != "set-B" {
				t.Fatalf("overlapping moves: %+v %+v", out, info)
			}
			updated := out.Items[0].MediaItem
			updated.Edition, updated.RatingKey = "Director", "new-key"
			if info := db.ReconcileMediaItemEdition(ctx, "1", "B", "", updated); info.Message != "" {
				t.Fatal(info)
			}
			out, info = db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1, ItemLibraryTitle: "B"})
			if info.Message != "" || len(out.Items) != 1 || out.Items[0].MediaItem.Edition != "Director" || len(out.Items[0].PosterSets) != 1 {
				t.Fatalf("edition reconciliation: %+v %+v", out, info)
			}
		})
	}
}
