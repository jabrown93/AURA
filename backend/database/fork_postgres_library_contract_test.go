package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
)

type libraryContractDB interface {
	SaveLibraryItems(context.Context, []models.MediaItem, int64) logging.LogErrorInfo
	GetLibraryItemsOnServer(context.Context) ([]models.MediaItem, logging.LogErrorInfo)
	FinishLibraryLoad(context.Context, []string, map[string]int64) logging.LogErrorInfo
	GetAllMediaItems(context.Context) ([]models.MediaItem, logging.LogErrorInfo)
	AutoIgnoreMediaItem(context.Context, string, string, string) logging.LogErrorInfo
	GetLibraryDBStates(context.Context, string) (map[string]MediaItemDBState, logging.LogErrorInfo)
	GetAllMediaItemsWithFlags(context.Context) ([]MediaItemWithFlags, logging.LogErrorInfo)
}

// TestLibraryStorageContract verifies library cleanup never discards saved or ignored state.
func TestLibraryStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db libraryContractDB
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
			items := []models.MediaItem{
				{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "100", Type: "movie", Title: "Ignored", Guids: []models.MediaItemGuid{{Provider: "tmdb", ID: "1"}}, AddedAt: 42},
				{TMDB_ID: "2", LibraryTitle: "Movies", RatingKey: "200", Type: "movie", Title: "Gone"},
			}
			if info := db.SaveLibraryItems(ctx, items, 100); info.Message != "" {
				t.Fatal(info)
			}
			listed, info := db.GetLibraryItemsOnServer(ctx)
			if info.Message != "" || len(listed) != 2 || listed[0].AddedAt != 42 || len(listed[0].Guids) != 1 {
				t.Fatalf("library round trip: %+v %+v", listed, info)
			}
			if info := db.AutoIgnoreMediaItem(ctx, "1", "Movies", ""); info.Message != "" {
				t.Fatal(info)
			}
			if info := db.FinishLibraryLoad(ctx, []string{"Movies"}, map[string]int64{"Movies": 200}); info.Message != "" {
				t.Fatal(info)
			}
			listed, info = db.GetLibraryItemsOnServer(ctx)
			if info.Message != "" || len(listed) != 0 {
				t.Fatalf("gone items still on server: %+v %+v", listed, info)
			}
			retained, info := db.GetAllMediaItems(ctx)
			if info.Message != "" || len(retained) != 1 || retained[0].TMDB_ID != "1" {
				t.Fatalf("ignored item lost: %+v %+v", retained, info)
			}
			states, info := db.GetLibraryDBStates(ctx, "Movies")
			if info.Message != "" || !states[dbStateKey("1", "")].Ignored {
				t.Fatalf("ignored library state lost: %+v %+v", states, info)
			}
			flags, info := db.GetAllMediaItemsWithFlags(ctx)
			if info.Message != "" || len(flags) != 1 || !flags[0].IsIgnored || flags[0].HasSavedSet {
				t.Fatalf("library flags: %+v %+v", flags, info)
			}
		})
	}
}
