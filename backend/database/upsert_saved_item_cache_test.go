package database

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
)

type upsertClearsIgnoreDB struct {
	DB
}

func (*upsertClearsIgnoreDB) UpsertSavedItem(context.Context, models.DBSavedItem) logging.LogErrorInfo {
	return logging.LogErrorInfo{}
}

func (*upsertClearsIgnoreDB) CheckIfMediaItemExists(context.Context, string, string) (bool, string, []models.DBSavedSet, logging.LogErrorInfo) {
	return false, "", nil, logging.LogErrorInfo{}
}

func TestUpsertSavedItemClearsCachedIgnoreState(t *testing.T) {
	previousDB := Client
	previousCache := cache.LibraryStore
	t.Cleanup(func() {
		Client = previousDB
		cache.LibraryStore = previousCache
	})
	Client = &upsertClearsIgnoreDB{}
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.ReplaceAllSections([]*models.LibrarySection{{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{{
			TMDB_ID: "550", RatingKey: "rating-550", LibraryTitle: "Movies",
			IgnoredInDB: true, IgnoredMode: "always", IgnoredSets: []string{"set-1"},
		}},
	}}, 0)

	if Err := UpsertSavedItem(context.Background(), models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "550", LibraryTitle: "Movies"},
	}); Err.Message != "" {
		t.Fatalf("UpsertSavedItem returned %q", Err.Message)
	}

	item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if !found {
		t.Fatal("item missing from cache after upsert")
	}
	if item.IgnoredInDB || item.IgnoredMode != "" || len(item.IgnoredSets) != 0 {
		t.Fatalf("cached ignore state = %v/%q/%v; want cleared, since the write drops the IgnoredItems row",
			item.IgnoredInDB, item.IgnoredMode, item.IgnoredSets)
	}
}
