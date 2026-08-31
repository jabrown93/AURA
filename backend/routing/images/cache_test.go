package routes_images

import (
	"aura/cache"
	"aura/config"
	"aura/mediux"
	"aura/models"
	"net/http"
	"net/http/httptest"
	"testing"
)

const expectedImageCacheControl = "private, max-age=86400"

func TestSuccessfulImageResponsesAreBrowserCacheable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

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

	config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: server.URL, ApiToken: "plex-secret"}
	config.Current.Mediux.ApiToken = "mediux-secret"
	config.Current.Images.CacheImages.Enabled = false
	mediux.MediuxApiURL = server.URL

	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "media-1", Title: "Movie"}},
	})
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
	cache.CollectionsStore.UpsertCollection(&models.CollectionItem{
		RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection",
	})

	tests := []struct {
		name    string
		target  string
		handler http.HandlerFunc
	}{
		{"media item", "/api/images/media/item?rating_key=media-1&image_type=poster", GetMediaItemImage},
		{"collection", "/api/images/media/collection?rating_key=collection-1&image_type=poster", GetCollectionItemImage},
		{"MediUX image", "/api/images/mediux/item?asset_id=asset-1&modified_date=2024-01-01T12:00:00Z", GetMediuxImage},
		{"MediUX avatar", "/api/images/mediux/avatar?avatar_id=avatar-1", GetMediuxAvatarImage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodGet, test.target, nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Header().Get("Cache-Control"); got != expectedImageCacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, expectedImageCacheControl)
			}
		})
	}
}

func TestImageErrorResponsesAreNotCacheable(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		handler http.HandlerFunc
	}{
		{"media item", "/api/images/media/item", GetMediaItemImage},
		{"collection", "/api/images/media/collection", GetCollectionItemImage},
		{"MediUX image", "/api/images/mediux/item?quality=invalid", GetMediuxImage},
		{"MediUX avatar", "/api/images/mediux/avatar", GetMediuxAvatarImage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodGet, test.target, nil))

			if got := response.Header().Get("Cache-Control"); got != "" {
				t.Errorf("error response Cache-Control = %q, want no cache policy", got)
			}
		})
	}
}
