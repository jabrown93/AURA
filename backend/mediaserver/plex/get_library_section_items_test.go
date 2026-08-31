package plex

import (
	"aura/config"
	"aura/logging"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func TestFetchLatestEpisodeAddedAtByShowUsesBoundedPages(t *testing.T) {
	const totalEpisodes = 2001
	var mu sync.Mutex
	requestedSizes := []int{}
	requestedStarts := []int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		size, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Size"))
		start, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Start"))
		mu.Lock()
		requestedSizes = append(requestedSizes, size)
		requestedStarts = append(requestedStarts, start)
		mu.Unlock()

		metadata := make([]PlexLibraryItemsMetadata, 0, size)
		if size > 0 {
			end := min(start+size, totalEpisodes)
			for i := start; i < end; i++ {
				metadata = append(metadata, PlexLibraryItemsMetadata{
					GrandParentRatingKey: fmt.Sprintf("show-%d", i%2),
					AddedAt:              int64(i + 1),
				})
			}
		}
		_ = json.NewEncoder(w).Encode(PlexLibraryItemsWrapper{MediaContainer: PlexLibraryItems{
			TotalSize: totalEpisodes,
			Metadata:  metadata,
		}})
	}))
	defer server.Close()

	previous := config.Current.MediaServer
	config.Current.MediaServer.URL = server.URL
	t.Cleanup(func() { config.Current.MediaServer = previous })

	ctx, logData := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, logData.AddAction("episode pages", logging.LevelDebug))
	latest, logErr := fetchLatestEpisodeAddedAtByShow(ctx, "1")
	if logErr.Message != "" {
		t.Fatalf("fetchLatestEpisodeAddedAtByShow() error = %s", logErr.Message)
	}
	if latest["show-0"] != 2001 || latest["show-1"] != 2000 {
		t.Fatalf("latest = %+v, want show-0=2001 and show-1=2000", latest)
	}
	for _, size := range requestedSizes {
		if size > 1000 {
			t.Fatalf("requested container sizes = %v; size %d is unbounded", requestedSizes, size)
		}
	}
	wantStarts := []int{0, 0, 1000, 2000}
	if fmt.Sprint(requestedStarts) != fmt.Sprint(wantStarts) {
		t.Fatalf("requested starts = %v, want %v", requestedStarts, wantStarts)
	}
}
