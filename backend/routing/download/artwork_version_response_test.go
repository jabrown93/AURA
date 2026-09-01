package routes_download

import (
	"aura/cache"
	"aura/config"
	"aura/mediux"
	"aura/models"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestArtworkApplyResponsesReturnAdvancedVersionOnCacheMiss(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanupArtworkApplyResponseTest(t, server.URL)
	baseVersion := time.Now().Add(time.Second).UnixMicro()

	mediaItem := models.MediaItem{
		RatingKey: "movie-1", Type: "movie", Title: "Movie", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: baseVersion,
	}
	mediaResponse := postArtworkApply[DownloadImageFileForMediaItem_Response](t, "/api/download/image/item", map[string]any{
		"media_item": mediaItem,
		"image_file": models.ImageFile{ID: "poster", Type: "poster"},
	}, DownloadImageFileForMediaItem)
	if mediaResponse.Status != "success" {
		t.Fatalf("media response status = %q, want success", mediaResponse.Status)
	}
	if mediaResponse.Data.ArtworkVersion != baseVersion+1 {
		t.Fatalf("media response artwork_version = %d, want %d", mediaResponse.Data.ArtworkVersion, baseVersion+1)
	}

	collection := models.CollectionItem{
		RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection", UpdatedAt: 100, ArtworkVersion: baseVersion,
	}
	collectionResponse := postArtworkApply[DownloadCollectionImage_Response](t, "/api/download/image/collection", map[string]any{
		"collection_item": collection,
		"image_file":      models.ImageFile{ID: "collection-poster", Type: "collection_poster"},
	}, DownloadImageFileForCollectionItem)
	if collectionResponse.Status != "success" {
		t.Fatalf("collection response status = %q, want success", collectionResponse.Status)
	}
	if collectionResponse.Data.ArtworkVersion != baseVersion+1 {
		t.Fatalf("collection response artwork_version = %d, want %d", collectionResponse.Data.ArtworkVersion, baseVersion+1)
	}
}

func TestFailedArtworkApplyResponsesDoNotReturnVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "apply failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	cleanupArtworkApplyResponseTest(t, server.URL)
	baseVersion := time.Now().Add(time.Second).UnixMicro()
	mediaItem := models.MediaItem{
		RatingKey: "movie-1", Type: "movie", Title: "Movie", LibraryTitle: "Movies", UpdatedAt: 100, ArtworkVersion: baseVersion,
	}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{mediaItem},
	})

	response := postArtworkApply[map[string]any](t, "/api/download/image/item", map[string]any{
		"media_item": mediaItem,
		"image_file": models.ImageFile{ID: "poster", Type: "poster"},
	}, DownloadImageFileForMediaItem)
	if _, ok := response.Data["artwork_version"]; ok {
		t.Fatal("failed apply response returned artwork_version")
	}
	if cached, _ := cache.LibraryStore.GetMediaItemByRatingKey(mediaItem.RatingKey); cached.UpdatedAt != 100 || cached.ArtworkVersion != baseVersion {
		t.Fatalf("failed apply changed item: %+v", cached)
	}

	collection := models.CollectionItem{
		RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection", UpdatedAt: 100, ArtworkVersion: baseVersion,
	}
	cache.CollectionsStore.UpsertCollection(&collection)
	collectionResponse := postArtworkApply[map[string]any](t, "/api/download/image/collection", map[string]any{
		"collection_item": collection,
		"image_file":      models.ImageFile{ID: "collection-poster", Type: "collection_poster"},
	}, DownloadImageFileForCollectionItem)
	if _, ok := collectionResponse.Data["artwork_version"]; ok {
		t.Fatal("failed collection apply response returned artwork_version")
	}
	if cached, _ := cache.CollectionsStore.GetCollectionByRatingKey(collection.RatingKey); cached.UpdatedAt != 100 || cached.ArtworkVersion != baseVersion {
		t.Fatalf("failed collection apply changed item: %+v", cached)
	}
}

func postArtworkApply[T any](t *testing.T, path string, body any, handler http.HandlerFunc) struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
} {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusOK && recorder.Code != http.StatusInternalServerError {
		t.Fatalf("response status code = %d", recorder.Code)
	}
	var response struct {
		Status string `json:"status"`
		Data   T      `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}

func cleanupArtworkApplyResponseTest(t *testing.T, serverURL string) {
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
