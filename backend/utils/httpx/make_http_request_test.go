package httpx

import (
	"aura/logging"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMakeHTTPRequestHonorsCallerContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelTrace))

	ctx, cancel := context.WithCancel(ctx)
	cancel()

	_, _, errInfo := MakeHTTPRequest(ctx, server.URL, http.MethodGet, nil, 60, nil, "Test")

	if errInfo.Message == "" {
		t.Fatal("MakeHTTPRequest succeeded on a cancelled caller context, so the caller context is being discarded")
	}
}

// net/http rewrites a password-bearing URL to "user:***@host" before storing it on the
// url.Error, so the stored URL no longer matches the raw one a substring replace looks for.
