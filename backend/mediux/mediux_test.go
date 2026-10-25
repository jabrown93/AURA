package mediux

import (
	"aura/config"
	"aura/logging"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

// useFakeMediux points the MediUX client at a fake server for the test
func useFakeMediux(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	previousURL, previousConfig := MediuxApiURL, config.Current
	t.Cleanup(func() {
		server.Close()
		MediuxApiURL, config.Current = previousURL, previousConfig
	})
	MediuxApiURL = server.URL
	config.Current.Mediux.ApiToken = "token"
}

func TestConstructImageUrl(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Mediux.DownloadQuality = "original"
	modified := "2025-03-04T05:06:07Z"

	url, Err := ConstructImageUrl(testContext(), "asset", modified, ImageQualityOriginal)
	if Err.Message != "" || url != MediuxApiURL+"/assets/asset?v=20250304050607" {
		t.Errorf("original = %q, %s", url, Err.Message)
	}
	if url, _ := ConstructImageUrl(testContext(), "asset", modified, ImageQualityThumb); !strings.HasSuffix(url, "&key=thumb") {
		t.Errorf("thumb = %q", url)
	}
	// Optimized downloads are set in the config
	config.Current.Mediux.DownloadQuality = "optimized"
	if url, _ := ConstructImageUrl(testContext(), "asset", modified, ImageQualityOriginal); !strings.HasSuffix(url, "&key=jpg") {
		t.Errorf("optimized = %q", url)
	}
	// Manual imports start with ---
	if url, _ := ConstructImageUrl(testContext(), "---manual", modified, ImageQualityOriginal); url != MediuxPublicURL+"/assets/manual?v=20250304050607" {
		t.Errorf("manual import = %q", url)
	}
	if _, Err := ConstructImageUrl(testContext(), "", modified, ImageQualityOriginal); Err.Message == "" {
		t.Error("an empty asset ID should fail")
	}
	if _, Err := ConstructImageUrl(testContext(), "asset", modified, "huge"); Err.Message == "" {
		t.Error("an unknown quality should fail")
	}

	if got := GetImageURLFromSrc("abc"); got != MediuxApiURL+"/assets/abc&key=thumb" {
		t.Errorf("GetImageURLFromSrc = %q", got)
	}
	if GetImageURLFromSrc("") != "" {
		t.Error("an empty src should have no URL")
	}
}

func TestAddMediuxAuthHeader(t *testing.T) {
	headers := AddMediuxAuthHeader(MediuxApiURL+"/graphql", "token", false, map[string]string{})
	if headers["Authorization"] != "Bearer token" || headers["Accept"] != "application/json" {
		t.Errorf("headers = %v", headers)
	}
	// The token is only sent to MediUX
	headers = AddMediuxAuthHeader("https://example.com/image", "token", true, map[string]string{})
	if headers["Authorization"] != "" || headers["Accept"] != "image/*" {
		t.Errorf("headers for another site = %v", headers)
	}
}

func TestConvertMediuxShowImages(t *testing.T) {
	blurhash := "LKO2"
	modified := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	set := BaseMediuxShowSet{
		ShowPoster:    []ImageAsset{{ID: "p", ModifiedOn: modified, Filesize: "100", Blurhash: &blurhash}},
		SeasonPosters: []ImageAsset{{ID: "s1", Season: &Season{SeasonNumber: 1}, Language: Language{DisplayName: "French"}}},
		Titlecards:    []ImageAsset{{ID: "t", Episode: &Episode{EpisodeTitle: "Pilot", EpisodeNumber: 2, Season: Season{SeasonNumber: 1}}}, {ID: ""}},
	}

	images := convertMediuxShowImagesToImageFiles(set, "10")
	if len(images) != 3 {
		t.Fatalf("got %d images, want 3 (an asset without an ID is skipped)", len(images))
	}
	poster, season, titlecard := images[0], images[1], images[2]
	if poster.Type != "poster" || poster.FileSize != 100 || poster.Blurhash != blurhash || poster.ItemTMDB_ID != "10" || poster.Language != "English" {
		t.Errorf("poster = %+v", poster)
	}
	if season.Type != "season_poster" || *season.SeasonNumber != 1 || season.Language != "French" {
		t.Errorf("season poster = %+v", season)
	}
	if titlecard.Type != "titlecard" || titlecard.Title != "Pilot" || *titlecard.SeasonNumber != 1 || *titlecard.EpisodeNumber != 2 {
		t.Errorf("titlecard = %+v", titlecard)
	}
}

func TestValidateToken(t *testing.T) {
	useFakeMediux(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me" || r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))

	if valid, Err := ValidateToken(testContext(), "good"); !valid || Err.Message != "" {
		t.Errorf("a good token = %v, %s", valid, Err.Message)
	}
	if valid, _ := ValidateToken(testContext(), "bad"); valid {
		t.Error("a bad token should not be valid")
	}
	if _, Err := ValidateToken(testContext(), ""); Err.Message == "" {
		t.Error("an empty token should fail")
	}
}

func TestSearchTMDBIDByTVDBID(t *testing.T) {
	useFakeMediux(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body MediuxGraphQLQueryBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer token" || body.Variables["tvdb_id"] != "81189" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"shows":[{"id":"1396","tvdb_id":"81189"}]}}`))
	}))

	tmdbID, found, Err := SearchTMDBIDByTVDBID(testContext(), "81189", "show")
	if Err.Message != "" || !found || tmdbID != "1396" {
		t.Errorf("SearchTMDBIDByTVDBID = %q, %v, %s", tmdbID, found, Err.Message)
	}
	if _, found, _ := SearchTMDBIDByTVDBID(testContext(), "81189", "episode"); found {
		t.Error("an unknown item type should not be found")
	}
}
