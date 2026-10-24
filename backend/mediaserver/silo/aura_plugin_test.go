package silo

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeSiloAuraPlugin is a Silo server with the aura plugin in library 6 only
type fakeSiloAuraPlugin struct {
	mu            sync.Mutex
	lockedFields  []int
	patched       [][]int
	applied       int
	providerCalls int
}

func newFakeSiloAuraPlugin(t *testing.T, lockedFields []int) (*fakeSiloAuraPlugin, *Silo) {
	t.Helper()
	f := &fakeSiloAuraPlugin{lockedFields: lockedFields}
	mux := http.NewServeMux()
	// The config only has library titles, the IDs come from Silo
	mux.HandleFunc("GET /api/v2/libraries", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"3","name":"4K Movies","type":"movies","enabled":true},{"id":"6","name":"Movies","type":"movies","enabled":true}]}`))
	})
	mux.HandleFunc("GET /api/v2/admin/plugins/installations", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"2","plugin_id":"silo.tmdb","enabled":true},{"id":"20","plugin_id":"community.aura","enabled":true}]}`))
	})
	mux.HandleFunc("GET /api/v2/libraries/{id}/providers", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.providerCalls++
		f.mu.Unlock()
		if r.PathValue("id") == "6" {
			_, _ = w.Write([]byte(`{"levels":[{"content_level":"movie","entries":[{"plugin_installation_id":"1","enabled":true},{"plugin_installation_id":"20","enabled":true},{"plugin_installation_id":"2","enabled":true}]}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"levels":[{"content_level":"movie","entries":[{"plugin_installation_id":"2","enabled":true}]}]}`))
	})
	mux.HandleFunc("GET /api/v2/catalog/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Profile-Id") == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"content_id": r.PathValue("id"), "locked_fields": f.lockedFields})
	})
	mux.HandleFunc("PATCH /api/v2/admin/items/{id}/metadata", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			LockedFields []int `json:"locked_fields"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.patched = append(f.patched, req.LockedFields)
		f.lockedFields = req.LockedFields
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("POST /api/v2/admin/items/{id}/images/apply", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.applied++
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"content_id":"x","stored_path":"x","thumbhash":"x"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	auraPluginChecksMu.Lock()
	auraPluginChecks = map[string]auraPluginCheck{}
	auraPluginChecksMu.Unlock()

	s := &Silo{Config: config.Config_MediaServer{ID: "silo", Type: "Silo", URL: server.URL, UserID: "profile",
		Libraries: []models.LibrarySection{
			{LibrarySectionBase: models.LibrarySectionBase{Title: "4K Movies", Type: "movie"}},
			{LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"}},
		}}}
	return f, s
}

func TestAuraPluginActive(t *testing.T) {
	f, s := newFakeSiloAuraPlugin(t, nil)
	ctx := testContext()

	if !auraPluginActive(ctx, s, "Movies") {
		t.Error("the plugin is in the Movies providers, it should be active there")
	}
	if auraPluginActive(ctx, s, "4K Movies") {
		t.Error("the plugin isn't in the 4K Movies providers")
	}
	if auraPluginActive(ctx, s, "Not a library") {
		t.Error("a library aura doesn't have can't use the plugin")
	}

	// The checks are kept for a minute
	auraPluginActive(ctx, s, "Movies")
	if f.providerCalls != 2 {
		t.Errorf("asked Silo for providers %d times, want 2", f.providerCalls)
	}
	previousNow := now
	t.Cleanup(func() { now = previousNow })
	now = func() time.Time { return time.Now().Add(2 * auraPluginCheckTTL) }
	auraPluginActive(ctx, s, "Movies")
	if f.providerCalls != 3 {
		t.Errorf("asked Silo for providers %d times after a minute, want 3", f.providerCalls)
	}
}

func TestUnlockImages(t *testing.T) {
	f, s := newFakeSiloAuraPlugin(t, []int{5, 10})
	ctx := testContext()

	if Err := UnlockImages(ctx, s.Config, "movie-tmdb-1"); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if len(f.patched) != 1 || !slices.Equal(f.patched[0], []int{5}) {
		t.Errorf("patched locked fields = %v, want [[5]] (only the images lock removed)", f.patched)
	}

	// Already unlocked, nothing is sent
	if Err := UnlockImages(ctx, s.Config, "movie-tmdb-1"); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if len(f.patched) != 1 {
		t.Errorf("patched %d times, want 1", len(f.patched))
	}
}

func TestDownloadApplyImageWithAuraPlugin(t *testing.T) {
	f, s := newFakeSiloAuraPlugin(t, []int{10})
	previous := struct {
		delay time.Duration
		jobs  func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo)
		save  bool
	}{refreshDelay, getQueueJobs, config.Current.Images.SaveImagesLocally.Enabled}
	t.Cleanup(func() {
		// Stop the timers and let a running one finish before putting things back
		pendingRefreshesMu.Lock()
		for key, timer := range pendingRefreshes {
			timer.Stop()
			delete(pendingRefreshes, key)
		}
		pendingRefreshesMu.Unlock()
		time.Sleep(50 * time.Millisecond)
		refreshDelay, getQueueJobs = previous.delay, previous.jobs
		config.Current.Images.SaveImagesLocally.Enabled = previous.save
	})
	refreshDelay = time.Hour
	config.Current.Images.SaveImagesLocally.Enabled = false

	item := &models.MediaItem{TMDB_ID: "1", Type: "movie", Title: "Hope", LibraryTitle: "Movies", RatingKey: "movie-tmdb-1"}
	image := models.ImageFile{ID: "00000000-0000-4000-8000-000000000001", Type: "poster", Modified: time.Now()}
	if Err := s.DownloadApplyImageToMediaItem(testContext(), item, image); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if f.applied != 0 {
		t.Errorf("applied %d images in Silo, want 0, the plugin gives Silo the image on a refresh", f.applied)
	}
	pendingRefreshesMu.Lock()
	_, pending := pendingRefreshes["movie-tmdb-1"]
	pendingRefreshesMu.Unlock()
	if !pending {
		t.Error("the item should have a refresh waiting")
	}

	// While the item is in the Download Queue the refresh waits
	refreshDelay = 10 * time.Millisecond
	getQueueJobs = func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return []database.DownloadQueueJob{{TMDB_ID: "1", Status: "processing"}}, logging.LogErrorInfo{}
	}
	pendingRefreshesMu.Lock()
	pendingRefreshes["movie-tmdb-1"].Reset(refreshDelay)
	pendingRefreshesMu.Unlock()
	time.Sleep(50 * time.Millisecond)
	f.mu.Lock()
	patched := len(f.patched)
	f.mu.Unlock()
	if patched != 0 {
		t.Error("the images shouldn't be unlocked while the item is still in the Download Queue")
	}
}
