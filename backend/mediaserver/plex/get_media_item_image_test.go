package plex

import (
	"aura/config"
	"aura/logging"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetImageFromPlexVersionsSourcePathWithArtworkState(t *testing.T) {
	requests := make(chan *http.Request, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("image"))
	}))
	defer server.Close()

	previous := config.Current.MediaServer
	config.Current.MediaServer = config.Config_MediaServer{
		Type:     "Plex",
		URL:      server.URL,
		ApiToken: "plex-secret",
	}
	t.Cleanup(func() { config.Current.MediaServer = previous })

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelTrace))

	for _, updatedAt := range []int64{1700000000, 1700000000, 1700000001} {
		image, errInfo := GetImageFromPlex(ctx, "123", "poster", updatedAt)
		if errInfo.Message != "" {
			t.Fatalf("GetImageFromPlex() error = %q", errInfo.Message)
		}
		if string(image) != "image" {
			t.Fatalf("GetImageFromPlex() image = %q, want %q", image, "image")
		}
	}

	wantPaths := []string{
		"/library/metadata/123/poster/1700000000",
		"/library/metadata/123/poster/1700000000",
		"/library/metadata/123/poster/1700000001",
	}
	for _, wantPath := range wantPaths {
		request := <-requests
		if got := request.URL.Query().Get("url"); got != wantPath {
			t.Errorf("Plex source path = %q, want %q", got, wantPath)
		}
		if got := request.Header.Get("X-Plex-Token"); got != "plex-secret" {
			t.Errorf("X-Plex-Token = %q, want token sent in header", got)
		}
		if request.URL.Query().Has("X-Plex-Token") {
			t.Error("Plex token must not appear in transcode URL")
		}
	}
}
