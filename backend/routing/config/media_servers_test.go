package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testLibraries(titles ...string) []models.LibrarySection {
	var sections []models.LibrarySection
	for _, title := range titles {
		sections = append(sections, models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: title}})
	}
	return sections
}

// fakePlexServer answers the Plex connection check and counts the requests
func fakePlexServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer": {"friendlyName": "Test Plex", "version": "1.40.0"}}`))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func TestCheckConfigDifferencesMediaServers(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	current := config.DefaultConfig()
	current.MediaServers = []config.Config_MediaServer{
		{ID: "main", Name: "Plex", Enabled: true, Type: "Plex", URL: "http://plex:32400", ApiToken: "plex-token", Libraries: testLibraries("Movies")},
		{ID: "second", Name: "Silo", Enabled: true, Type: "Silo", URL: "http://silo:8090", ApiToken: "silo-token", Libraries: testLibraries("Movies"), UserID: "profile-id"},
		{ID: "paused", Name: "Jellyfin", Enabled: false, Type: "Jellyfin", URL: "http://jellyfin:8096", ApiToken: "jf-token"},
	}
	current.MainMediaServer = "main"
	current.LoadMainMediaServer()
	current.SyncMediaServers()

	// The list sent back with masked tokens is not a change, and the connections aren't checked again
	sent := *current.SanitizeConfig(ctx)
	sent.MediaServer = current.MediaServer
	changed, valid := checkConfigDifferences_MediaServers(ctx, current, &sent)
	if changed || !valid {
		t.Errorf("changed %v, valid %v, want no change", changed, valid)
	}
	if sent.MediaServers[1].ApiToken != "silo-token" || sent.MediaServers[1].UserID != "profile-id" {
		t.Errorf("masked token and User ID should be kept: %+v", sent.MediaServers[1])
	}

	// Changing a paused media server is a change, and it isn't checked
	sent.MediaServers[2].URL = "http://jellyfin:9096"
	if changed, valid := checkConfigDifferences_MediaServers(ctx, current, &sent); !changed || !valid {
		t.Errorf("changed %v, valid %v, want a valid change", changed, valid)
	}

	// Another media server must have valid settings
	sent = current.Clone()
	sent.MediaServers[1].URL = "silo:8090"
	if _, valid := checkConfigDifferences_MediaServers(ctx, current, &sent); valid {
		t.Error("a URL without http:// should be invalid")
	}
}

func TestKeepMediaServerIDs(t *testing.T) {
	current := []config.Config_MediaServer{
		{ID: "main", Type: "Plex", URL: "http://plex:32400/"},
		{ID: "second", Type: "Silo", URL: "http://silo:8090", LibraryMap: map[string]string{"Films": "Movies"}},
	}
	// An imported config file from before multiple media servers gets new IDs
	imported := config.Config{
		MediaServers: []config.Config_MediaServer{
			{ID: "new-1", Type: "Plex", URL: "http://plex:32400"},
			{ID: "new-2", Type: "Silo", URL: "http://silo:8090"},
			{ID: "new-3", Type: "Plex", URL: "http://other:32400"},
		},
		MainMediaServer: "new-1",
		MediaServer:     config.Config_MediaServer{ID: "new-1"},
	}
	keepMediaServerIDs(current, &imported)

	if ids := []string{imported.MediaServers[0].ID, imported.MediaServers[1].ID, imported.MediaServers[2].ID}; ids[0] != "main" || ids[1] != "second" || ids[2] != "new-3" {
		t.Errorf("IDs = %v, want [main second new-3]", ids)
	}
	if imported.MainMediaServer != "main" || imported.MediaServer.ID != "main" {
		t.Errorf("MainMediaServer = %q, MediaServer.ID = %q, want main", imported.MainMediaServer, imported.MediaServer.ID)
	}
	if imported.MediaServers[1].LibraryMap["Films"] != "Movies" {
		t.Error("the library map should be kept")
	}
}

var refreshes atomic.Int32

func setupMediaServersTest(t *testing.T) (*memoryConfigStore, *atomic.Int32, string) {
	t.Helper()
	store := setupConfigSectionTest(t)
	plex, requests := fakePlexServer(t)

	// Count the library refreshes instead of running them, and don't start the Plex event listener
	previousRefresh, previousListener := refreshOtherMediaServers, restartPlexEventListener
	t.Cleanup(func() { refreshOtherMediaServers, restartPlexEventListener = previousRefresh, previousListener })
	refreshOtherMediaServers = func() { refreshes.Add(1) }
	restartPlexEventListener = func() {}
	refreshes.Store(0)

	cfg := config.Current.Clone()
	cfg.MediaServers = []config.Config_MediaServer{
		{ID: "main", Name: "Plex", Enabled: true, Type: "Plex", URL: plex.URL, ApiToken: "plex-token", Libraries: testLibraries("Movies", "Series")},
	}
	cfg.MainMediaServer = "main"
	cfg.LoadMainMediaServer()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	if Err := cfg.Save(ctx); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	config.Current = cfg
	store.saves = 0
	return store, requests, plex.URL
}

func TestMediaServersSectionAddsAServer(t *testing.T) {
	store, requests, plexURL := setupMediaServersTest(t)

	body := `{"media_servers": [
		{"id": "main", "name": "Plex", "enabled": true, "type": "Plex", "url": "` + plexURL + `", "api_token": "***oken", "libraries": [{"title": "Movies"}, {"title": "Series"}]},
		{"name": "Second Plex", "enabled": true, "type": "plex", "url": "` + plexURL + `/", "api_token": "second-token", "libraries": [{"title": "Movies"}, {"title": "Films"}]}
	]}`
	code, response, errorMessage := patchConfigSection(t, "media_servers", body)
	if code != http.StatusOK || errorMessage != "" || !response.Changed {
		t.Fatalf("status %d, error %q, changed %v", code, errorMessage, response.Changed)
	}

	servers := config.Current.MediaServers
	if len(servers) != 2 || config.Current.MainMediaServer != "main" {
		t.Fatalf("MediaServers = %+v", servers)
	}
	main, second := servers[0], servers[1]
	if main.ApiToken != "plex-token" {
		t.Errorf("the masked token should be kept, got %q", main.ApiToken)
	}
	if second.ID == "" || second.Type != "Plex" || strings.HasSuffix(second.URL, "/") || second.ApiToken != "second-token" {
		t.Errorf("the new media server should get an ID and cleaned up settings: %+v", second)
	}
	if second.LibraryMap["Movies"] != "Movies" || second.LibraryMap["Films"] != "" && len(second.LibraryMap) != 1 {
		t.Errorf("LibraryMap = %v, want Movies matched by title", second.LibraryMap)
	}
	// Only the new media server's connection is checked, and its libraries are loaded
	if got := requests.Load(); got != 1 {
		t.Errorf("connection checks = %d, want 1", got)
	}
	waitForRefreshes(t, 1)
	if store.saves != 1 || !strings.Contains(store.values["MediaServers"], "second-token") || strings.Contains(store.values["MediaServers"], "***") {
		t.Errorf("saves %d, stored MediaServers = %s", store.saves, store.values["MediaServers"])
	}
	// The response masks the tokens
	for _, server := range response.Status.CurrentSetup.MediaServers {
		if !strings.HasPrefix(server.ApiToken, "***") {
			t.Errorf("token not masked in the response: %q", server.ApiToken)
		}
	}

	// Sending the same list back with masked tokens is not a change, so nothing is checked again
	requests.Store(0)
	body = `{"media_servers": [
		{"id": "main", "name": "Plex", "enabled": true, "type": "Plex", "url": "` + plexURL + `", "api_token": "***oken", "libraries": [{"title": "Movies"}, {"title": "Series"}]},
		{"id": "` + second.ID + `", "name": "Second Plex", "enabled": true, "type": "Plex", "url": "` + plexURL + `", "api_token": "***oken", "libraries": [{"title": "Movies"}, {"title": "Films"}], "library_map": {"Movies": "Movies"}}
	]}`
	if _, response, errorMessage := patchConfigSection(t, "media_servers", body); errorMessage != "" || response.Changed {
		t.Errorf("error %q, changed %v, want no change", errorMessage, response.Changed)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("connection checks = %d, want 0", got)
	}
	waitForRefreshes(t, 1)
}

// waitForRefreshes waits for the background library refreshes to start
func waitForRefreshes(t *testing.T, want int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for refreshes.Load() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if got := refreshes.Load(); got != want {
		t.Errorf("library refreshes = %d, want %d", got, want)
	}
}

func TestMediaServersSectionRejectsChanges(t *testing.T) {
	_, _, plexURL := setupMediaServersTest(t)
	before := config.Current.Clone()

	tests := map[string]struct{ body, wantError string }{
		"main server change": {
			`{"main_media_server": "other", "media_servers": [{"id": "other", "type": "Plex", "url": "` + plexURL + `", "api_token": "x", "libraries": [{"title": "Movies"}]}]}`,
			"The main media server is changed with Make Main",
		},
		"other config field": {`{"auth": {"enabled": true}}`, "Invalid changes for config section 'media_servers'"},
		"main server removed": {
			`{"media_servers": [{"id": "other", "enabled": true, "type": "Plex", "url": "` + plexURL + `", "api_token": "x", "libraries": [{"title": "Movies"}]}]}`,
			"The main media server can't be removed",
		},
		"invalid server": {
			`{"media_servers": [{"id": "main", "type": "Plex", "url": "` + plexURL + `", "api_token": "***oken", "libraries": [{"title": "Movies"}, {"title": "Series"}]},
				{"name": "Broken", "enabled": true, "type": "Kodi", "url": "http://kodi", "api_token": "x", "libraries": [{"title": "Movies"}]}]}`,
			"Media server 'Broken'",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, errorMessage := patchConfigSection(t, "media_servers", test.body)
			if !strings.Contains(errorMessage, test.wantError) {
				t.Errorf("error %q, want %q", errorMessage, test.wantError)
			}
			if !configEqual(before, config.Current) {
				t.Error("the config should not change")
			}
		})
	}
}

func TestMediaServersSectionKeepsTheMainServer(t *testing.T) {
	_, requests, plexURL := setupMediaServersTest(t)

	// An older copy of the main media server doesn't change it
	body := `{"media_servers": [{"id": "main", "name": "Old Name", "enabled": true, "type": "Plex", "url": "` + plexURL + `", "api_token": "***oken", "libraries": [{"title": "Movies"}]}]}`
	_, response, errorMessage := patchConfigSection(t, "media_servers", body)
	if errorMessage != "" || response.Changed {
		t.Errorf("error %q, changed %v, want no change", errorMessage, response.Changed)
	}
	if main := config.Current.MediaServers[0]; main.Name != "Plex" || len(main.Libraries) != 2 || requests.Load() != 0 {
		t.Errorf("main media server = %+v, connection checks %d", main, requests.Load())
	}
}

func TestMediaServerSectionRenamesTheMainServer(t *testing.T) {
	store, _, _ := setupMediaServersTest(t)

	if _, response, errorMessage := patchConfigSection(t, "media_server", `{"name": "Living Room"}`); errorMessage != "" || !response.Changed {
		t.Fatalf("error %q, changed %v", errorMessage, response.Changed)
	}
	if config.Current.MediaServers[0].Name != "Living Room" || config.Current.MediaServer.Name != "Living Room" {
		t.Errorf("name = %q / %q, want Living Room", config.Current.MediaServers[0].Name, config.Current.MediaServer.Name)
	}
	if !strings.Contains(store.values["MediaServers"], `"Name":"Living Room"`) {
		t.Errorf("stored MediaServers = %s", store.values["MediaServers"])
	}
}
