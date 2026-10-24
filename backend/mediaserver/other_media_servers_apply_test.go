package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"sync"
	"testing"
)

// fakeMediaServer records what is applied, instead of calling a media server
type fakeMediaServer struct {
	MediaServerInterface
	server  config.Config_MediaServer
	calls   *[]string
	details *int
	mu      *sync.Mutex
}

func (f fakeMediaServer) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*f.calls = append(*f.calls, f.server.ID+": "+call)
}

func (f fakeMediaServer) GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (bool, logging.LogErrorInfo) {
	f.mu.Lock()
	*f.details++
	f.mu.Unlock()
	return true, logging.LogErrorInfo{}
}

func (f fakeMediaServer) DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) logging.LogErrorInfo {
	f.record("image " + item.RatingKey)
	return logging.LogErrorInfo{}
}

func (f fakeMediaServer) RateMediaItem(ctx context.Context, item *models.MediaItem, rating float64) logging.LogErrorInfo {
	f.record("rating " + item.RatingKey)
	return logging.LogErrorInfo{}
}

func (f fakeMediaServer) AddLabelToMediaItem(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) logging.LogErrorInfo {
	f.record("labels " + item.RatingKey)
	return logging.LogErrorInfo{}
}

func movieSection(title string, items ...models.MediaItem) *models.LibrarySection {
	return &models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: title}, MediaItems: items}
}

func setupOtherMediaServersTest(t *testing.T) (calls *[]string, details *int) {
	t.Helper()
	previousConfig, previousClient := config.Current, newOtherMediaServerClient
	t.Cleanup(func() {
		config.Current, newOtherMediaServerClient = previousConfig, previousClient
		cache.KeepServerLibraryStores(nil)
		clearOtherMediaItems()
	})

	cfg := config.DefaultConfig()
	cfg.MediaServers = []config.Config_MediaServer{
		{ID: "main", Name: "Plex", Enabled: true, Type: "Plex"},
		{ID: "silo", Name: "Silo", Enabled: true, Type: "Silo", UserID: "profile",
			LibraryMap: map[string]string{"Films": "Movies", "4K": "4K Movies", "Kids": ""}},
		{ID: "plex2", Name: "Plex 2", Enabled: true, Type: "Plex", LibraryMap: map[string]string{"Movies": "Movies"}},
		{ID: "jellyfin", Name: "Jellyfin", Enabled: true, Type: "Jellyfin", LibraryMap: map[string]string{"Movies": "Movies"}},
		{ID: "paused", Name: "Paused", Enabled: false, Type: "Plex", LibraryMap: map[string]string{"Movies": "Movies"}},
	}
	cfg.MainMediaServer = "main"
	cfg.LoadMainMediaServer()
	config.Current = cfg

	cache.ServerLibraryStore("silo").UpdateSection(movieSection("Films",
		models.MediaItem{TMDB_ID: "1", Type: "movie", RatingKey: "movie-tmdb-1", LibraryTitle: "Films"},
		models.MediaItem{TMDB_ID: "5", Type: "show", RatingKey: "show-tmdb-5", LibraryTitle: "Films"},
	))
	cache.ServerLibraryStore("silo").UpdateSection(movieSection("4K",
		models.MediaItem{TMDB_ID: "2", Type: "movie", RatingKey: "movie-tmdb-2", LibraryTitle: "4K"},
	))
	cache.ServerLibraryStore("silo").UpdateSection(movieSection("Kids",
		models.MediaItem{TMDB_ID: "3", Type: "movie", RatingKey: "movie-tmdb-3", LibraryTitle: "Kids"},
	))
	cache.ServerLibraryStore("plex2").UpdateSection(movieSection("Movies",
		models.MediaItem{TMDB_ID: "1", Type: "movie", RatingKey: "999", LibraryTitle: "Movies", Edition: "Directors Cut"},
		models.MediaItem{TMDB_ID: "1", Type: "movie", RatingKey: "998", LibraryTitle: "Movies"},
	))

	calls, details = &[]string{}, new(int)
	mu := &sync.Mutex{}
	newOtherMediaServerClient = func(server *config.Config_MediaServer) (MediaServerInterface, logging.LogErrorInfo) {
		return fakeMediaServer{server: *server, calls: calls, details: details, mu: mu}, logging.LogErrorInfo{}
	}
	return calls, details
}

func applyImage(item models.MediaItem) {
	applyToOtherMediaServers(item, "the poster image", nil, func(ctx context.Context, msClient MediaServerInterface, otherItem *models.MediaItem) logging.LogErrorInfo {
		return msClient.DownloadApplyImageToMediaItem(ctx, otherItem, models.ImageFile{Type: "poster"})
	})
}

func TestApplyToOtherMediaServers(t *testing.T) {
	calls, details := setupOtherMediaServersTest(t)

	applyImage(models.MediaItem{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies", RatingKey: "100"})
	// Silo through its library map, Plex 2 by library title. Jellyfin isn't connected yet and Paused is turned off.
	want := []string{"silo: image movie-tmdb-1", "plex2: image 998"}
	if !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}

	// The items found are kept, so the next image doesn't look them up again
	applyImage(models.MediaItem{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies", RatingKey: "100"})
	if *details != 2 || len(*calls) != 4 {
		t.Errorf("details lookups = %d, calls = %v, want 2 lookups and 4 calls", *details, *calls)
	}
}

func TestApplyToOtherMediaServersEditions(t *testing.T) {
	calls, _ := setupOtherMediaServersTest(t)

	// The matching edition is used when the media server has it
	applyImage(models.MediaItem{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies", Edition: "Directors Cut"})
	// Otherwise the base item, as Silo has no editions
	applyImage(models.MediaItem{TMDB_ID: "2", Type: "movie", LibraryTitle: "4K Movies", Edition: "Imax"})

	want := []string{"silo: image movie-tmdb-1", "plex2: image 999", "silo: image movie-tmdb-2"}
	if !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}

func TestApplyToOtherMediaServersNotFound(t *testing.T) {
	calls, _ := setupOtherMediaServersTest(t)

	// A show with the same TMDB ID as a movie is a different item
	applyImage(models.MediaItem{TMDB_ID: "1", Type: "show", LibraryTitle: "Movies"})
	// A library that doesn't match any library on the other media servers
	applyImage(models.MediaItem{TMDB_ID: "5", Type: "show", LibraryTitle: "Series"})
	// Kids doesn't match any main server library, so it isn't searched
	applyImage(models.MediaItem{TMDB_ID: "3", Type: "movie", LibraryTitle: ""})

	if len(*calls) != 0 {
		t.Errorf("calls = %v, want none", *calls)
	}
}

func TestRatingsAndLabelsOnOtherMediaServers(t *testing.T) {
	calls, _ := setupOtherMediaServersTest(t)
	item := models.MediaItem{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies"}

	applyToOtherMediaServers(item, "the rating", supportsRating, func(ctx context.Context, msClient MediaServerInterface, otherItem *models.MediaItem) logging.LogErrorInfo {
		return msClient.RateMediaItem(ctx, otherItem, 8)
	})
	applyToOtherMediaServers(item, "labels", isPlex, func(ctx context.Context, msClient MediaServerInterface, otherItem *models.MediaItem) logging.LogErrorInfo {
		return msClient.AddLabelToMediaItem(ctx, *otherItem, models.SelectedTypes{})
	})

	// Ratings on Plex and Silo, labels on Plex only
	want := []string{"silo: rating movie-tmdb-1", "plex2: rating 998", "plex2: labels 998"}
	if !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}
