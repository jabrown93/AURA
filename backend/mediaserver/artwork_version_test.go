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
	baseVersion := time.Now().Add(time.Second).UnixMicro()
	seasonNumber, episodeNumber := 1, 2
	item := models.MediaItem{
		RatingKey: "show-1", Type: "show", Title: "Show", LibraryTitle: "TV", UpdatedAt: 100, ArtworkVersion: baseVersion,
		Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{
			RatingKey: "season-1", SeasonNumber: seasonNumber,
			Episodes: []models.MediaItemEpisode{{RatingKey: "episode-2", SeasonNumber: seasonNumber, EpisodeNumber: episodeNumber}},
		}}},
	}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{item},
	})
	collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", Title: "Collection", UpdatedAt: 100, ArtworkVersion: baseVersion}
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
		if !ok || cached.UpdatedAt != 100 || cached.ArtworkVersion != baseVersion+int64(i)+1 {
			t.Fatalf("after %s cached item = %+v, found = %v; want timestamp 100 and version %d", image.Type, cached, ok, baseVersion+int64(i)+1)
		}
		if item.UpdatedAt != 100 || item.ArtworkVersion != cached.ArtworkVersion {
			t.Fatalf("after %s mutation result = %+v, want timestamp 100 and cached version %d", image.Type, item, cached.ArtworkVersion)
		}
	}

	collectionImage := models.ImageFile{ID: "collection", Type: "collection_poster", Modified: time.Unix(1, 0)}
	if err := ApplyCollectionImage(testLogContext(), &collection, collectionImage); err.Message != "" {
		t.Fatalf("collection apply failed: %s", err.Message)
	}
	cachedCollection, ok := cache.CollectionsStore.GetCollectionByRatingKey(collection.RatingKey)
	if !ok || cachedCollection.UpdatedAt != 100 || cachedCollection.ArtworkVersion != baseVersion+1 {
		t.Fatalf("cached collection = %+v, found = %v; want timestamp 100 and version %d", cachedCollection, ok, baseVersion+1)
	}
	if collection.UpdatedAt != 100 || collection.ArtworkVersion != cachedCollection.ArtworkVersion {
		t.Fatalf("collection mutation result = %+v, want timestamp 100 and cached version %d", collection, cachedCollection.ArtworkVersion)
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

func TestSuccessfulArtworkAppliesAdvanceUncachedParentVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanupArtworkVersionTest(t, server.URL)
	baseVersion := time.Now().Add(time.Second).UnixMicro()
	item := models.MediaItem{RatingKey: "movie-1", Type: "movie", Title: "Movie", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: baseVersion}
	image := models.ImageFile{ID: "poster", Type: "poster", Modified: time.Unix(1, 0)}

	if err := DownloadApplyImageToMediaItem(testLogContext(), &item, image); err.Message != "" {
		t.Fatalf("media apply failed: %s", err.Message)
	}
	if item.UpdatedAt != 100 || item.ArtworkVersion != baseVersion+1 {
		t.Fatalf("uncached media item = %+v, want timestamp 100 and version %d", item, baseVersion+1)
	}

	collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: baseVersion}
	image.Type = "collection_poster"
	if err := ApplyCollectionImage(testLogContext(), &collection, image); err.Message != "" {
		t.Fatalf("collection apply failed: %s", err.Message)
	}
	if collection.UpdatedAt != 100 || collection.ArtworkVersion != baseVersion+1 {
		t.Fatalf("uncached collection = %+v, want timestamp 100 and version %d", collection, baseVersion+1)
	}
}

func TestFailedArtworkAppliesDoNotAdvanceVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "apply failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	cleanupArtworkVersionTest(t, server.URL)
	version := time.Now().Add(time.Second).UnixMicro()
	item := models.MediaItem{RatingKey: "movie-1", Type: "movie", Title: "Movie", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: version}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{item},
	})
	collection := models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: version}
	cache.CollectionsStore.UpsertCollection(&collection)
	image := models.ImageFile{ID: "image", Type: "poster", Modified: time.Unix(1, 0)}

	if err := DownloadApplyImageToMediaItem(testLogContext(), &item, image); err.Message == "" {
		t.Fatal("media apply unexpectedly succeeded")
	}
	if cached, _ := cache.LibraryStore.GetMediaItemByRatingKey(item.RatingKey); cached.UpdatedAt != 100 || cached.ArtworkVersion != version {
		t.Fatalf("failed media apply changed item: %+v", cached)
	}
	image.Type = "collection_poster"
	if err := ApplyCollectionImage(testLogContext(), &collection, image); err.Message == "" {
		t.Fatal("collection apply unexpectedly succeeded")
	}
	if cached, _ := cache.CollectionsStore.GetCollectionByRatingKey(collection.RatingKey); cached.UpdatedAt != 100 || cached.ArtworkVersion != version {
		t.Fatalf("failed collection apply changed item: %+v", cached)
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
