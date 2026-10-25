package silo

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

// fakeSiloCollections is a Silo server with a Mission: Impossible collection in library 3
type fakeSiloCollections struct {
	mu          sync.Mutex
	contentType string
	sourceURL   string
}

func newFakeSiloCollections(t *testing.T) (*fakeSiloCollections, *Silo) {
	t.Helper()
	f := &fakeSiloCollections{}
	mux := http.NewServeMux()
	// The config only has library titles, the IDs come from Silo
	mux.HandleFunc("GET /api/v2/libraries", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"3","name":"4K Movies","type":"movies","paths":["/movies"],"enabled":true}]}`))
	})
	mux.HandleFunc("GET /api/v2/admin/collections", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("library_id") != "3" {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[
			{"id":"147","title":"Mission: Impossible","description":"Ethan Hunt","item_count":2,"library_id":"3",
			 "poster_url":"/api/v2/artwork/collection-images/147/poster/w300.webp?exp=1&sig=a","backdrop_url":"",
			 "source_config":{"mode":"tmdb_collection","collection_id":87359}},
			{"id":"148","title":"Star Wars Saga","item_count":0,"library_id":"3","poster_url":"","backdrop_url":"","source_config":null}]}`))
	})
	mux.HandleFunc("GET /api/v2/library/3/collections/147/items", func(w http.ResponseWriter, r *http.Request) {
		// Two pages
		if r.URL.Query().Get("cursor") == "" {
			_, _ = w.Write([]byte(`{"items":[{"content_id":"movie-tmdb-575265","type":"movie","title":"The Final Reckoning"}],"page":{"has_more":true,"next_cursor":"next"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"content_id":"movie-tmdb-575264","type":"movie","title":"Dead Reckoning"},{"content_id":"movie-tmdb-1","type":"movie","title":"Not in aura"}],"page":{"has_more":false}}`))
	})
	mux.HandleFunc("GET /api/v2/artwork/collection-images/147/poster/w300.webp", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sig") != "a" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("poster bytes"))
	})
	mux.HandleFunc("PUT /api/v2/admin/collections/147/poster", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.contentType = r.Header.Get("Content-Type")
		f.sourceURL = r.FormValue("source_url")
		_, _ = w.Write([]byte(`{"id":"147"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	previous := config.Current
	t.Cleanup(func() {
		config.Current = previous
		cache.LibraryStore.ClearAllSections()
		cache.CollectionsStore.ClearAllCollections()
	})
	config.Current.MainMediaServer = "main"
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{ID: "3", Title: "4K Movies", Type: "movie"},
		MediaItems: []models.MediaItem{
			{RatingKey: "movie-tmdb-575265", TMDB_ID: "575265", Type: "movie", Title: "The Final Reckoning", LibraryTitle: "4K Movies"},
			{RatingKey: "movie-tmdb-575264", TMDB_ID: "575264", Type: "movie", Title: "Dead Reckoning", LibraryTitle: "4K Movies"},
		},
	})
	return f, &Silo{Config: config.Config_MediaServer{ID: "main", Type: "Silo", URL: server.URL, ApiToken: "token",
		Libraries: []models.LibrarySection{{LibrarySectionBase: models.LibrarySectionBase{Title: "4K Movies"}}}}}
}

func TestSiloCollections(t *testing.T) {
	fake, s := newFakeSiloCollections(t)
	ctx := testContext()
	library := models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{ID: "3", Title: "4K Movies", Type: "movie"}}

	collections, Err := s.GetMovieCollections(ctx, library)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	// The empty collection is left out
	if len(collections) != 1 {
		t.Fatalf("collections = %+v, want only Mission: Impossible", collections)
	}
	collection := collections[0]
	if collection.RatingKey != "147" || collection.TMDB_ID != "87359" || collection.ChildCount != 2 || collection.LibraryTitle != "4K Movies" {
		t.Errorf("collection = %+v", collection)
	}
	if _, found := cache.CollectionsStore.GetCollectionByRatingKey("147"); !found {
		t.Error("the collection should be in the collections cache")
	}

	// The movies come from both pages, matched to the library by Silo's content ID
	if Err := s.GetMovieCollectionChildrenItems(ctx, &collection); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if len(collection.MediaItems) != 2 || collection.MediaItems[0].TMDB_ID != "575265" || collection.MediaItems[1].TMDB_ID != "575264" {
		t.Errorf("collection movies = %+v, want the two in the library", collection.MediaItems)
	}

	poster, Err := s.GetCollectionItemImage(ctx, &collection, "poster")
	if Err.Message != "" || string(poster) != "poster bytes" {
		t.Errorf("poster = %q, %s", poster, Err.Message)
	}
	if _, Err := s.GetCollectionItemImage(ctx, &collection, "backdrop"); Err.Message == "" {
		t.Error("a collection without a backdrop should say so")
	}

	// Silo downloads the image from MediUX
	image := models.ImageFile{ID: "asset", Type: "collection_poster", Modified: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)}
	if Err := s.ApplyCollectionImage(ctx, &collection, image); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if !strings.HasPrefix(fake.contentType, "multipart/form-data") || !strings.Contains(fake.sourceURL, "asset") {
		t.Errorf("apply sent %q with source_url %q", fake.contentType, fake.sourceURL)
	}
}
