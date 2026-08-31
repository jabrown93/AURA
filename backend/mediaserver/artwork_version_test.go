package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSuccessfulArtworkAppliesAdvanceParentVersions(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanupArtworkVersionTest(t, server.URL)
	baseVersion := time.Now().UnixMicro() + 1000
	seasonNumber, episodeNumber := 1, 2
	item := models.MediaItem{
		RatingKey: "show-1", Type: "show", Title: "Show", LibraryTitle: "TV", UpdatedAt: baseVersion,
		Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{
			RatingKey: "season-1", SeasonNumber: seasonNumber,
			Episodes: []models.MediaItemEpisode{{RatingKey: "episode-2", SeasonNumber: seasonNumber, EpisodeNumber: episodeNumber}},
		}}},
	}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{item},
	})
	collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", Title: "Collection", UpdatedAt: baseVersion}
	cache.CollectionsStore.UpsertCollection(&collection)

	images := []models.ImageFile{
		{ID: "poster", Type: "poster", Modified: time.Unix(1, 0)},
		{ID: "season", Type: "season_poster", Modified: time.Unix(1, 0), SeasonNumber: &seasonNumber},
		{ID: "episode", Type: "titlecard", Modified: time.Unix(1, 0), SeasonNumber: &seasonNumber, EpisodeNumber: &episodeNumber},
	}
	for i, image := range images {
		if err := DownloadApplyImageToMediaItem(testLogContext(), &item, image); err.Message != "" {
			t.Fatalf("apply %s failed: %s", image.Type, err.Message)
		}
		cached, ok := cache.LibraryStore.GetMediaItemByRatingKey(item.RatingKey)
		if !ok || cached.UpdatedAt != baseVersion+int64(i)+1 {
			t.Fatalf("after %s parent version = %d, found = %v; want %d", image.Type, cached.UpdatedAt, ok, baseVersion+int64(i)+1)
		}
		if item.UpdatedAt != cached.UpdatedAt {
			t.Fatalf("after %s mutation result version = %d, want cached version %d", image.Type, item.UpdatedAt, cached.UpdatedAt)
		}
	}

	collectionImage := models.ImageFile{ID: "collection", Type: "collection_poster", Modified: time.Unix(1, 0)}
	if err := ApplyCollectionImage(testLogContext(), &collection, collectionImage); err.Message != "" {
		t.Fatalf("collection apply failed: %s", err.Message)
	}
	cachedCollection, ok := cache.CollectionsStore.GetCollectionByRatingKey(collection.RatingKey)
	if !ok || cachedCollection.UpdatedAt != baseVersion+1 {
		t.Fatalf("collection version = %d, found = %v; want %d", cachedCollection.UpdatedAt, ok, baseVersion+1)
	}
	if collection.UpdatedAt != cachedCollection.UpdatedAt {
		t.Fatalf("collection mutation result version = %d, want cached version %d", collection.UpdatedAt, cachedCollection.UpdatedAt)
	}

	mu.Lock()
	defer mu.Unlock()
	wantPaths := []string{
		"/library/metadata/show-1/posters",
		"/library/metadata/season-1/posters",
		"/library/metadata/episode-2/posters",
		"/library/metadata/collection-1/posters",
	}
	if len(paths) != len(wantPaths) {
		t.Fatalf("Plex request paths = %v, want %v", paths, wantPaths)
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Errorf("Plex request path %d = %q, want %q", i, paths[i], wantPaths[i])
		}
	}
}

func TestFailedArtworkAppliesDoNotAdvanceVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "apply failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	cleanupArtworkVersionTest(t, server.URL)
	version := time.Now().UnixMicro() + 1000
	item := models.MediaItem{RatingKey: "movie-1", Type: "movie", Title: "Movie", LibraryTitle: "Movies", UpdatedAt: version}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{item},
	})
	collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: version}
	cache.CollectionsStore.UpsertCollection(&collection)
	image := models.ImageFile{ID: "image", Type: "poster", Modified: time.Unix(1, 0)}

	if err := DownloadApplyImageToMediaItem(testLogContext(), &item, image); err.Message == "" {
		t.Fatal("media apply unexpectedly succeeded")
	}
	if cached, _ := cache.LibraryStore.GetMediaItemByRatingKey(item.RatingKey); cached.UpdatedAt != version {
		t.Fatalf("failed media apply advanced version to %d", cached.UpdatedAt)
	}
	image.Type = "collection_poster"
	if err := ApplyCollectionImage(testLogContext(), &collection, image); err.Message == "" {
		t.Fatal("collection apply unexpectedly succeeded")
	}
	if cached, _ := cache.CollectionsStore.GetCollectionByRatingKey(collection.RatingKey); cached.UpdatedAt != version {
		t.Fatalf("failed collection apply advanced version to %d", cached.UpdatedAt)
	}
}

func cleanupArtworkVersionTest(t *testing.T, serverURL string) {
	t.Helper()
	previousConfig := config.Current
	previousLibraryStore := cache.LibraryStore
	previousCollectionsStore := cache.CollectionsStore
	previousMediuxURL := mediux.MediuxApiURL
	t.Cleanup(func() {
		config.Current = previousConfig
		cache.LibraryStore = previousLibraryStore
		cache.CollectionsStore = previousCollectionsStore
		mediux.MediuxApiURL = previousMediuxURL
	})

	config.Current = config.Config{}
	config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: serverURL, ApiToken: "token"}
	mediux.MediuxApiURL = serverURL
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
}

func testLogContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "artwork version test")
	return logging.WithCurrentAction(ctx, ld.AddAction("apply", logging.LevelDebug))
}
