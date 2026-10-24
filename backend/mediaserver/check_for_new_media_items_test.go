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

func TestCheckForNewMediaItemsStopsAtAKnownItem(t *testing.T) {
	// Movies 5 to 1, newest first. aura already has 1 and 2
	var mu sync.Mutex
	starts := []int{}
	sorts := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /library/sections/all", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie","title":"Movies"}]}}`))
	})
	mux.HandleFunc("GET /library/sections/1/all", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Start"))
		size, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Size"))
		mu.Lock()
		starts = append(starts, start)
		sorts = append(sorts, r.URL.Query().Get("sort"))
		mu.Unlock()
		movies := []string{}
		for i := start; i < min(start+size, 5); i++ {
			id := 5 - i
			movies = append(movies, fmt.Sprintf(`{"ratingKey":"%d","type":"movie","title":"Movie %d","guid":"plex://movie/%d","Guid":[{"id":"tmdb://%d"}],"Media":[{"Part":[{"file":"/m/%d.mkv"}]}]}`, id, id, id, id, id))
		}
		_, _ = fmt.Fprintf(w, `{"MediaContainer":{"title1":"Movies","totalSize":5,"Metadata":[%s]}}`, strings.Join(movies, ","))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	previousConfig, previousPageSize := config.Current, newItemsPageSize
	t.Cleanup(func() {
		config.Current, newItemsPageSize = previousConfig, previousPageSize
		cache.LibraryStore.ClearAllSections()
	})
	newItemsPageSize = 2
	config.Current.MainMediaServer = "main"
	config.Current.MediaServer = config.Config_MediaServer{ID: "main", Name: "Plex", Type: "Plex", URL: server.URL,
		Libraries: []models.LibrarySection{{LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Movies", Type: "movie"}}}}
	cache.LibraryStore.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Movies", Type: "movie"},
		MediaItems:         []models.MediaItem{{TMDB_ID: "1", LibraryTitle: "Movies"}, {TMDB_ID: "2", LibraryTitle: "Movies"}},
	})

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	added, Err := CheckForNewMediaItems(ctx)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}

	if added != 3 || cache.LibraryStore.GetItemsCount() != 5 {
		t.Errorf("added %d, %d in the cache, want 3 and 5", added, cache.LibraryStore.GetItemsCount())
	}
	// The second page has a movie aura already has, so the check stops there
	if !slices.Equal(starts, []int{0, 2}) || sorts[0] != "addedAt:desc" {
		t.Errorf("pages started at %v sorted by %v, want [0 2] newest first", starts, sorts)
	}
}
