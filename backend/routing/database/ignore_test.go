package routes_db

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"net/http/httptest"
	"testing"
)

type successfulIgnoreDB struct{ database.DB }

func (*successfulIgnoreDB) IgnoreMediaItem(context.Context, string, string, string, string) logging.LogErrorInfo {
	return logging.LogErrorInfo{}
}

func (*successfulIgnoreDB) StopIgnoringMediaItem(context.Context, string, string) logging.LogErrorInfo {
	return logging.LogErrorInfo{}
}

func TestIgnoreHandlersSynchronizeLibraryCache(t *testing.T) {
	previousDB := database.Client
	previousCache := cache.LibraryStore
	t.Cleanup(func() {
		database.Client = previousDB
		cache.LibraryStore = previousCache
	})
	database.Client = &successfulIgnoreDB{}
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{{
			TMDB_ID: "550", DBSavedSets: []models.DBSavedSet{{ID: "set-1"}},
		}},
	})

	response := httptest.NewRecorder()
	IgnoreItemInDB(response, httptest.NewRequest("PATCH", "/api/db/ignore?tmdb_id=550&library_title=Movies&mode=always", nil))
	ignored, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if !ignored.IgnoredInDB || ignored.IgnoredMode != "always" || len(ignored.DBSavedSets) != 1 {
		t.Fatalf("ignore cache state = %+v", ignored)
	}

	response = httptest.NewRecorder()
	StopIgnoringItemInDB(response, httptest.NewRequest("PATCH", "/api/db/ignore/stop?tmdb_id=550&library_title=Movies", nil))
	unignored, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if unignored.IgnoredInDB || unignored.IgnoredMode != "" || len(unignored.DBSavedSets) != 1 {
		t.Fatalf("unignore cache state = %+v", unignored)
	}
}
