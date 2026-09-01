package routes_images

import (
	"aura/cache"
	"aura/config"
	"aura/mediux"
	"aura/models"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
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

	mediaVersion := time.Now().UnixMicro() + 1000
	collectionVersion := mediaVersion + 1
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "media-1", Title: "Movie", UpdatedAt: 100, ArtworkVersion: mediaVersion}},
	})
	cache.CollectionsStore = cache.Cache_NewCollectionsCache()
	cache.CollectionsStore.UpsertCollection(&models.CollectionItem{
		RatingKey: "collection-1", LibraryTitle: "Movies", Title: "Collection", UpdatedAt: 100, ArtworkVersion: collectionVersion,
	})

	tests := []struct {
		name         string
		target       string
		handler      http.HandlerFunc
		cacheControl string
	}{
		{"versioned media item", "/api/images/media/item?rating_key=media-1&image_type=poster&v=" + strconv.FormatInt(mediaVersion, 10), GetMediaItemImage, versionedImageCacheControl},
		{"unversioned media item", "/api/images/media/item?rating_key=media-1&image_type=poster", GetMediaItemImage, unversionedImageCacheControl},
		{"stale media item version", "/api/images/media/item?rating_key=media-1&image_type=poster&v=" + strconv.FormatInt(mediaVersion-1, 10), GetMediaItemImage, unversionedImageCacheControl},
		{"versioned collection", "/api/images/media/collection?rating_key=collection-1&image_type=poster&v=" + strconv.FormatInt(collectionVersion, 10), GetCollectionItemImage, versionedImageCacheControl},
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

func TestStaleBrowserVersionUsesCurrentPlexVersion(t *testing.T) {
	var upstreamVersions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamVersions = append(upstreamVersions, r.URL.Query().Get("url"))
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

	previousConfig := config.Current
	previousLibraryStore := cache.LibraryStore
	t.Cleanup(func() {
		config.Current = previousConfig
		cache.LibraryStore = previousLibraryStore
	})
	config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: server.URL, ApiToken: "plex-secret"}
	config.Current.Images.CacheImages.Enabled = false
	oldVersion := time.Now().UnixMicro() + 1000
	currentVersion := oldVersion + 1
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "media-1", UpdatedAt: 100, ArtworkVersion: oldVersion}},
	})
	if version, ok := cache.LibraryStore.AdvanceMediaItemArtworkVersion("media-1", currentVersion); !ok || version != currentVersion {
		t.Fatalf("advanced version = %d, found = %v; want %d, true", version, ok, currentVersion)
	}

	for _, test := range []struct {
		name         string
		version      string
		cacheControl string
	}{
		{"stale browser version", strconv.FormatInt(oldVersion, 10), unversionedImageCacheControl},
		{"current browser version", strconv.FormatInt(currentVersion, 10), versionedImageCacheControl},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/images/media/item?rating_key=media-1&image_type=poster&v="+test.version, nil)
			GetMediaItemImage(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Cache-Control"); got != test.cacheControl {
				t.Fatalf("Cache-Control = %q, want %q", got, test.cacheControl)
			}
		})
	}

	wantUpstream := "/library/metadata/media-1/poster/" + strconv.FormatInt(currentVersion, 10)
	for _, got := range upstreamVersions {
		if got != wantUpstream {
			t.Errorf("upstream Plex image version = %q, want %q", got, wantUpstream)
		}
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
