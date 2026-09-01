package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/models"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFullRefreshContinuesAfterMissingSectionWithoutPublishingFailure(t *testing.T) {
	var laterSectionRefreshed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/sections/all":
			_ = json.NewEncoder(w).Encode(map[string]any{"MediaContainer": map[string]any{
				"Directory": []map[string]any{{"key": "2", "type": "show", "title": "B Later"}},
			}})
		case "/library/sections/2/all":
			laterSectionRefreshed.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{"MediaContainer": map[string]any{
				"title1": "B Later", "totalSize": 1,
				"Metadata": []map[string]any{{
					"ratingKey": "later-new", "type": "show", "title": "Later New",
					"Guid": []map[string]any{{"id": "tmdb://2"}},
				}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousConfig := config.Current
	previousLibraryStore := cache.LibraryStore
	previousCollectionsStore := cache.CollectionsStore
	t.Cleanup(func() {
		config.Current = previousConfig
		cache.LibraryStore = previousLibraryStore
		cache.CollectionsStore = previousCollectionsStore
	})

	config.Current.MediaServer = config.Config_MediaServer{
		Type: "Plex", URL: server.URL, ApiToken: "token",
		Libraries: []models.LibrarySection{
			{LibrarySectionBase: models.LibrarySectionBase{Title: "A Missing"}},
			{LibrarySectionBase: models.LibrarySectionBase{Title: "B Later"}},
		},
	}
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
	cache.LibraryStore.LastFullUpdate = 101
	cache.CollectionsStore.LastFullUpdate = 202
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "A Missing", ID: "old"},
		MediaItems:         []models.MediaItem{{RatingKey: "missing-old", Title: "Missing Old"}},
	})

	success := getAllLibrarySectionsAndItemsImpl(testLogContext())

	if success {
		t.Error("full refresh succeeded after configured section was missing")
	}
	if !laterSectionRefreshed.Load() {
		t.Error("later section was not refreshed")
	}
	if _, found := cache.LibraryStore.GetMediaItemByRatingKey("later-new"); !found {
		t.Error("later section's new item was not cached")
	}
	if _, found := cache.LibraryStore.GetMediaItemByRatingKey("missing-old"); !found {
		t.Error("missing section's previous cache snapshot was replaced")
	}
	if got := cache.LibraryStore.LastFullUpdate; got != 101 {
		t.Errorf("library full-refresh timestamp = %d, want 101", got)
	}
	if got := cache.CollectionsStore.LastFullUpdate; got != 202 {
		t.Errorf("collections full-refresh timestamp = %d, want 202", got)
	}
}

func TestGetAllLibrarySectionsAndItemsSerializesInitialRefresh(t *testing.T) {
	warmupMu.Lock()
	oldRun := getAllLibrarySectionsAndItemsRun
	oldDone := warmupDone
	warmupDone = false
	warmupMu.Unlock()
	t.Cleanup(func() {
		warmupMu.Lock()
		getAllLibrarySectionsAndItemsRun = oldRun
		warmupDone = oldDone
		warmupMu.Unlock()
	})

	var calls atomic.Int32
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	getAllLibrarySectionsAndItemsRun = func(context.Context) bool {
		if calls.Add(1) == 1 {
			close(firstEntered)
			<-releaseFirst
		}
		return true
	}

	firstDone := make(chan struct{})
	go func() {
		GetAllLibrarySectionsAndItems(context.Background(), false)
		close(firstDone)
	}()
	<-firstEntered

	secondDone := make(chan struct{})
	go func() {
		GetAllLibrarySectionsAndItems(context.Background(), false)
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("second initial refresh overlapped first")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseFirst)
	<-firstDone
	<-secondDone

	if got := calls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
}
