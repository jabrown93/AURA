package routes_images

import (
	"aura/cache"
	"aura/config"
	"aura/mediux"
	"aura/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuccessfulImageResponsesUseSafeCachePolicies(t *testing.T) {
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
		MediaItems:         []models.MediaItem{{RatingKey: "media-1", Title: "Movie", UpdatedAt: 1700000000}},
	})
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
	cache.CollectionsStore.UpsertCollection(&models.CollectionItem{
		RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection", UpdatedAt: 1700000001,
	})

	tests := []struct {
		name         string
		target       string
		handler      http.HandlerFunc
		cacheControl string
	}{
		{"versioned media item", "/api/images/media/item?rating_key=media-1&image_type=poster&v=1700000000", GetMediaItemImage, versionedImageCacheControl},
		{"unversioned media item", "/api/images/media/item?rating_key=media-1&image_type=poster", GetMediaItemImage, unversionedImageCacheControl},
		{"stale media item version", "/api/images/media/item?rating_key=media-1&image_type=poster&v=1699999999", GetMediaItemImage, unversionedImageCacheControl},
		{"versioned collection", "/api/images/media/collection?rating_key=collection-1&image_type=poster&v=1700000001", GetCollectionItemImage, versionedImageCacheControl},
		{"unversioned collection", "/api/images/media/collection?rating_key=collection-1&image_type=poster", GetCollectionItemImage, unversionedImageCacheControl},
		{"versioned MediUX image", "/api/images/mediux/item?asset_id=asset-1&modified_date=2024-01-01T12:00:00Z", GetMediuxImage, versionedImageCacheControl},
		{"unversioned MediUX avatar", "/api/images/mediux/avatar?avatar_id=avatar-1", GetMediuxAvatarImage, unversionedImageCacheControl},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodGet, test.target, nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Header().Get("Cache-Control"); got != test.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, test.cacheControl)
			}
		})
	}
}

func TestImageErrorResponsesAreNotCacheable(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		handler    http.HandlerFunc
		bodyString string
	}{
		{"media item", "/api/images/media/item", GetMediaItemImage, "Missing Query Parameters"},
		{"collection", "/api/images/media/collection", GetCollectionItemImage, "Missing Query Parameters"},
		{"MediUX image", "/api/images/mediux/item?quality=invalid", GetMediuxImage, "Invalid quality parameter"},
		{"MediUX avatar", "/api/images/mediux/avatar", GetMediuxAvatarImage, "Missing avatar identifier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.handler(response, httptest.NewRequest(http.MethodGet, test.target, nil))

			if response.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d; body = %q", response.Code, http.StatusInternalServerError, response.Body.String())
			}
			if body := response.Body.String(); !strings.Contains(body, `"status":"error"`) || !strings.Contains(body, test.bodyString) {
				t.Errorf("body = %q, want error status and %q", body, test.bodyString)
			}
			if got := response.Header().Get("Cache-Control"); got != "" {
				t.Errorf("error response Cache-Control = %q, want no cache policy", got)
			}
		})
	}
}
