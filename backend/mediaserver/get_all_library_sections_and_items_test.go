package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
