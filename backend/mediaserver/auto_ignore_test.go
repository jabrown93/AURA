package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type autoIgnoreTestDB struct {
	database.DB
	mode     string
	checkErr logging.LogErrorInfo
	ignored  int
}

func (db *autoIgnoreTestDB) GetTempIgnoredItems(context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	return nil, logging.LogErrorInfo{}
}

func (db *autoIgnoreTestDB) CheckIfMediaItemExists(context.Context, string, string, string) (bool, string, []models.DBSavedSet, logging.LogErrorInfo) {
	return db.mode != "", db.mode, nil, db.checkErr
}

func (db *autoIgnoreTestDB) AutoIgnoreMediaItem(context.Context, string, string, string) logging.LogErrorInfo {
	db.ignored++
	db.mode = "until-set-available"
	return logging.LogErrorInfo{}
}

func TestAutoIgnoreItemsWithoutSets(t *testing.T) {
	for _, tc := range []struct {
		name, itemType, id, mode, response string
		enabled, dbError                   bool
		status, wantIgnored, wantRequests  int
	}{
		{name: "disabled", itemType: "movie", id: "1"},
		{name: "movie without sets", enabled: true, itemType: "movie", id: "1", wantIgnored: 1, wantRequests: 2},
		{name: "show without sets", enabled: true, itemType: "show", id: "1", wantIgnored: 1, wantRequests: 1},
		{name: "movie with sets", enabled: true, itemType: "movie", id: "1", response: `{"data":{"movies_by_id":{"id":"1","movie_sets":[{"id":"set1"}]}}}`, wantRequests: 1},
		{name: "show with sets", enabled: true, itemType: "show", id: "1", response: `{"data":{"shows_by_id":{"id":"1","show_sets":[{"id":"set1"}]}}}`, wantRequests: 1},
		{name: "collection sets", enabled: true, itemType: "movie", id: "1", response: `{"data":{"movies_by_id":{"collection_id":{"movies":[{"id":"1","posters":[{"id":"image1","collection_set":{"id":"set1"}}]}]}}}}`, wantRequests: 2},
		{name: "lookup failure", enabled: true, itemType: "movie", id: "1", status: 500, wantRequests: 1},
		{name: "collection lookup failure", enabled: true, itemType: "movie", id: "1", response: `{"errors":[{"message":"unavailable"}]}`, wantRequests: 2},
		{name: "manual ignore", enabled: true, itemType: "movie", id: "1", mode: "always"},
		{name: "new set ignore", enabled: true, itemType: "show", id: "1", mode: "until-new-set-available"},
		{name: "missing ID", enabled: true, itemType: "movie"},
		{name: "unsupported type", enabled: true, itemType: "episode", id: "1"},
		{name: "database failure", enabled: true, itemType: "show", id: "1", dbError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldConfig, oldDB, oldURL, oldCache := config.Current, database.Client, mediux.MediuxApiURL, cache.LibraryStore
			t.Cleanup(func() {
				config.Current, database.Client, mediux.MediuxApiURL, cache.LibraryStore = oldConfig, oldDB, oldURL, oldCache
			})
			db := &autoIgnoreTestDB{mode: tc.mode}
			if tc.dbError {
				db.checkErr.Message = "database unavailable"
			}
			database.Client = db
			config.Current.UserPreferences.AutoIgnoreItemsWithoutSets = tc.enabled
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					return
				}
				var query mediux.MediuxGraphQLQueryBody
				if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
					t.Error(err)
				}
				response := tc.response
				if (tc.name == "collection sets" || tc.name == "collection lookup failure") && query.QueryName != "getMovieItemCollectionSetsByTMDBID" {
					response = ""
				}
				if response == "" {
					response = `{"data":{}}`
				}
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			mediux.MediuxApiURL = server.URL
			cache.LibraryStore = cache.Cache_NewLibraryCache()
			cache.LibraryStore.UpdateSection(&models.LibrarySection{
				LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Library", Type: tc.itemType},
				MediaItems:         []models.MediaItem{{TMDB_ID: tc.id, Type: tc.itemType, LibraryTitle: "Library"}},
			})
			ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
			ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
			err := HandleTempIgnoredItems(ctx)
			wantError := tc.dbError || tc.status != 0 || tc.name == "collection lookup failure"
			if (err.Message != "") != wantError {
				t.Fatalf("error=%q, wantError=%v", err.Message, wantError)
			}
			if db.ignored != tc.wantIgnored || requests != tc.wantRequests {
				t.Fatalf("ignored=%d requests=%d, want %d/%d", db.ignored, requests, tc.wantIgnored, tc.wantRequests)
			}
			if tc.wantIgnored > 0 {
				HandleTempIgnoredItems(ctx)
				if db.ignored != 1 {
					t.Fatal("repeated scan duplicated ignore")
				}
			}
		})
	}
}
