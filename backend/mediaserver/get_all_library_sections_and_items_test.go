package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type pagedLibraryClient struct {
	MediaServerInterface
	failAt int
	starts []string
}

func (f *pagedLibraryClient) GetLibrarySectionItems(_ context.Context, section models.LibrarySection, start, _ string) ([]models.MediaItem, int, int, logging.LogErrorInfo) {
	f.starts = append(f.starts, start)
	if len(f.starts) == f.failAt {
		return nil, 1, 1001, logging.LogErrorInfo{Message: "page failed"}
	}
	item := models.MediaItem{
		TMDB_ID:      fmt.Sprintf("tmdb-%s", start),
		RatingKey:    fmt.Sprintf("rating-%s", start),
		LibraryTitle: section.Title,
	}
	rawItemCount := 1
	if start == "0" {
		rawItemCount = 1000
	}
	return []models.MediaItem{item}, rawItemCount, 1001, logging.LogErrorInfo{}
}

type filteredMiddlePageClient struct {
	MediaServerInterface
	starts []string
}

func (f *filteredMiddlePageClient) GetLibrarySectionItems(_ context.Context, section models.LibrarySection, start, _ string) ([]models.MediaItem, int, int, logging.LogErrorInfo) {
	f.starts = append(f.starts, start)
	if start == "1000" {
		return nil, 1000, 2001, logging.LogErrorInfo{}
	}
	return []models.MediaItem{{
		TMDB_ID: "tmdb-" + start, RatingKey: "rating-" + start, LibraryTitle: section.Title,
	}}, map[string]int{"0": 1000, "2000": 1}[start], 2001, logging.LogErrorInfo{}
}

func TestFetchSectionSnapshotContinuesAfterFilteredEmptyPage(t *testing.T) {
	client := &filteredMiddlePageClient{}

	snapshot, ok := fetchSectionSnapshot(context.Background(), client, models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
	}, map[database.MediaItemKey]database.MediaItemState{})

	if !ok || snapshot == nil || len(snapshot.MediaItems) != 2 || snapshot.TotalSize != 2 {
		t.Fatalf("snapshot = %+v, success = %v; want valid items from pages before and after filtered page", snapshot, ok)
	}
	if fmt.Sprint(client.starts) != "[0 1000 2000]" {
		t.Fatalf("page starts = %v, want raw offsets [0 1000 2000]", client.starts)
	}
}

func TestFetchSectionSnapshotPreservesCacheWhenLaterPageFails(t *testing.T) {
	previous := cache.LibraryStore
	t.Cleanup(func() { cache.LibraryStore = previous })
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{TMDB_ID: "old", RatingKey: "old-key"}},
	})
	client := &pagedLibraryClient{failAt: 2}

	snapshot, ok := fetchSectionSnapshot(context.Background(), client, models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
	}, map[database.MediaItemKey]database.MediaItemState{})

	if ok || snapshot != nil {
		t.Fatalf("fetchSectionSnapshot() = (%+v, %v), want nil, false", snapshot, ok)
	}
	if fmt.Sprint(client.starts) != "[0 1000]" {
		t.Fatalf("page starts = %v, want deterministic [0 1000]", client.starts)
	}
	section, _ := cache.LibraryStore.GetSectionByTitle("TV")
	if len(section.MediaItems) != 1 || section.MediaItems[0].TMDB_ID != "old" {
		t.Fatalf("failed fetch changed prior snapshot: %+v", section)
	}
}

func TestFetchSectionSnapshotAccumulatesAllPagesBeforeReturn(t *testing.T) {
	client := &pagedLibraryClient{}

	snapshot, ok := fetchSectionSnapshot(context.Background(), client, models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
	}, map[database.MediaItemKey]database.MediaItemState{})

	if !ok || snapshot == nil || len(snapshot.MediaItems) != 2 || snapshot.TotalSize != 2 {
		t.Fatalf("snapshot = %+v, success = %v; want complete two-item snapshot", snapshot, ok)
	}
	if fmt.Sprint(client.starts) != "[0 1000]" {
		t.Fatalf("page starts = %v, want deterministic [0 1000]", client.starts)
	}
}

func TestFullRefreshCommitsAllSectionsOnlyAfterSuccess(t *testing.T) {
	failSecondSection := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/sections/all":
			fmt.Fprint(w, `{"MediaContainer":{"Directory":[{"key":"1","type":"show","title":"A"},{"key":"2","type":"show","title":"B"}]}}`)
		case "/library/sections/1/all":
			fmt.Fprint(w, `{"MediaContainer":{"title1":"A","totalSize":1,"Metadata":[{"ratingKey":"new-a","type":"show","title":"A","Guid":[{"id":"tmdb://1"}]}]}}`)
		case "/library/sections/2/all":
			if failSecondSection {
				http.Error(w, "failed", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, `{"MediaContainer":{"title1":"B","totalSize":1,"Metadata":[{"ratingKey":"new-b","type":"show","title":"B","Guid":[{"id":"tmdb://2"}]}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousConfig := config.Current
	previousLibrary := cache.LibraryStore
	previousCollections := cache.CollectionsStore
	t.Cleanup(func() {
		config.Current = previousConfig
		cache.LibraryStore = previousLibrary
		cache.CollectionsStore = previousCollections
	})
	config.Current.MediaServer = config.Config_MediaServer{
		Type: "Plex", URL: server.URL,
		Libraries: []models.LibrarySection{
			{LibrarySectionBase: models.LibrarySectionBase{Title: "B"}},
			{LibrarySectionBase: models.LibrarySectionBase{Title: "A"}},
		},
	}
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
	cache.LibraryStore.ReplaceAllSections([]*models.LibrarySection{
		{LibrarySectionBase: models.LibrarySectionBase{Title: "A"}, MediaItems: []models.MediaItem{{TMDB_ID: "old-a"}}},
		{LibrarySectionBase: models.LibrarySectionBase{Title: "B"}, MediaItems: []models.MediaItem{{TMDB_ID: "old-b"}}},
		{LibrarySectionBase: models.LibrarySectionBase{Title: "Removed"}},
	}, 77)

	if getAllLibrarySectionsAndItemsImpl(testLogContext()) {
		t.Fatal("partial full refresh reported success")
	}
	if cache.LibraryStore.GetLastFullUpdate() != 77 {
		t.Fatalf("failed refresh timestamp = %d, want prior 77", cache.LibraryStore.GetLastFullUpdate())
	}
	if item, _ := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("A", "old-a"); item.TMDB_ID != "old-a" {
		t.Fatalf("failed refresh replaced earlier section: %+v", item)
	}
	if _, found := cache.LibraryStore.GetSectionByTitle("Removed"); !found {
		t.Fatal("failed refresh pruned prior section")
	}

	failSecondSection = false
	if !getAllLibrarySectionsAndItemsImpl(testLogContext()) {
		t.Fatal("complete full refresh reported failure")
	}
	if _, found := cache.LibraryStore.GetSectionByTitle("Removed"); found {
		t.Fatal("successful refresh retained de-configured section")
	}
	if _, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("A", "1"); !found {
		t.Fatal("successful refresh did not publish new section snapshot")
	}
}

type stateRereadDB struct {
	database.DB
	reads int
}

func (*stateRereadDB) UpdateMediaItemsOnServer(context.Context, string, []string, bool) logging.LogErrorInfo {
	return logging.LogErrorInfo{}
}

func (f *stateRereadDB) GetAllMediaItemStates(context.Context) (map[database.MediaItemKey]database.MediaItemState, logging.LogErrorInfo) {
	f.reads++
	if f.reads == 1 {
		return map[database.MediaItemKey]database.MediaItemState{}, logging.LogErrorInfo{}
	}
	return map[database.MediaItemKey]database.MediaItemState{
		{TMDBID: "550", LibraryTitle: "Movies"}: {Ignored: true, IgnoreMode: "always"},
	}, logging.LogErrorInfo{}
}

func TestPublishSectionSnapshotRereadsDatabaseStateForItemsMissingFromCache(t *testing.T) {
	previousDB := database.Client
	previousCache := cache.LibraryStore
	t.Cleanup(func() {
		database.Client = previousDB
		cache.LibraryStore = previousCache
	})
	db := &stateRereadDB{}
	database.Client = db
	cache.LibraryStore = cache.Cache_NewLibraryCache()

	generation := cache.LibraryStore.DBMutationGeneration()
	staged, ok := fetchSectionSnapshot(context.Background(), &singleItemClient{}, models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
	}, map[database.MediaItemKey]database.MediaItemState{})
	if !ok {
		t.Fatal("staging the section snapshot failed")
	}

	// The item is absent from the live cache, so the ignore made while the
	// snapshot was staged only exists in the database.
	db.reads = 1
	publishSectionSnapshot(context.Background(), staged, generation)

	published, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID("Movies", "550")
	if !found || !published.IgnoredInDB || published.IgnoredMode != "always" {
		t.Fatalf("published item = %+v, found = %v; want ignored state from the re-read", published, found)
	}
}

type singleItemClient struct{ MediaServerInterface }

func (*singleItemClient) GetLibrarySectionItems(_ context.Context, section models.LibrarySection, _, _ string) ([]models.MediaItem, int, int, logging.LogErrorInfo) {
	return []models.MediaItem{{
		TMDB_ID: "550", RatingKey: "rating-550", LibraryTitle: section.Title,
	}}, 1, 1, logging.LogErrorInfo{}
}

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
	cache.LibraryStore.ReplaceAllSections(nil, 101)
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
	if got := cache.LibraryStore.GetLastFullUpdate(); got != 101 {
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
