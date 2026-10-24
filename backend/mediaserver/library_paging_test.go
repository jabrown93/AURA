package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestLibraryPagesStartAfterSkippedItems(t *testing.T) {
	// Five movies, and the second has no TMDB ID so it is skipped
	var mu sync.Mutex
	starts := []int{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /library/sections/all", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie","title":"Movies"}]}}`))
	})
	mux.HandleFunc("GET /library/sections/1/all", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Start"))
		size, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Size"))
		mu.Lock()
		starts = append(starts, start)
		mu.Unlock()
		movies := []string{}
		for i := start; i < min(start+size, 5); i++ {
			guid := fmt.Sprintf(`,"guid":"plex://movie/%d","Guid":[{"id":"tmdb://%d"}]`, i, i)
			if i == 1 {
				guid = ""
			}
			movies = append(movies, fmt.Sprintf(`{"ratingKey":"%d","type":"movie","title":"Movie %d"%s,"Media":[{"Part":[{"file":"/m/%d.mkv"}]}]}`, i, i, guid, i))
		}
		_, _ = fmt.Fprintf(w, `{"MediaContainer":{"title1":"Movies","totalSize":5,"Metadata":[%s]}}`, strings.Join(movies, ","))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	previousConfig, previousPageSize := config.Current, libraryPageSize
	t.Cleanup(func() { config.Current, libraryPageSize = previousConfig, previousPageSize })
	config.Current.MainMediaServer = "main"
	libraryPageSize = 2

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	store := cache.Cache_NewLibraryCache()
	plexServer := config.Config_MediaServer{ID: "other", Name: "Plex", Type: "Plex", URL: server.URL,
		Libraries: []models.LibrarySection{{LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Movies", Type: "movie"}}}}
	if !loadLibrariesIntoCache(ctx, plexServer, store, false) {
		t.Fatal("loading the library failed")
	}

	if !slices.Equal(starts, []int{0, 2, 4}) {
		t.Errorf("pages started at %v, want [0 2 4]", starts)
	}
	if got := store.GetItemsCount(); got != 4 {
		t.Errorf("items = %d, want 4", got)
	}
}
