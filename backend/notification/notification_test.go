package notification

import (
	"aura/config"
	"aura/logging"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

// recordingServer answers with status and keeps the last request
func recordingServer(t *testing.T, status int) (*httptest.Server, *http.Request, *[]byte) {
	t.Helper()
	var last http.Request
	body := new([]byte)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = *r.Clone(context.Background())
		*body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server, &last, body
}

func TestSendWebhookMessage(t *testing.T) {
	server, req, body := recordingServer(t, http.StatusOK)
	provider := &config.Config_Notification_Webhook{URL: server.URL, Headers: map[string]string{"X-Token": "secret"}}

	if Err := SendWebhookMessage(testContext(), provider, "Hello", "https://img", "Title"); Err.Message != "" {
		t.Fatalf("SendWebhookMessage: %s", Err.Message)
	}
	var payload map[string]string
	_ = json.Unmarshal(*body, &payload)
	if payload["title"] != "Title" || payload["message"] != "Hello" || payload["image_url"] != "https://img" {
		t.Errorf("payload = %v", payload)
	}
	if req.Header.Get("X-Token") != "secret" {
		t.Error("the configured headers should be sent")
	}

	failing, _, _ := recordingServer(t, http.StatusInternalServerError)
	if Err := SendWebhookMessage(testContext(), &config.Config_Notification_Webhook{URL: failing.URL}, "Hello", "", "Title"); Err.Message == "" {
		t.Error("a non-2xx response should fail")
	}
	if Err := SendWebhookMessage(testContext(), &config.Config_Notification_Webhook{}, "Hello", "", "Title"); Err.Message == "" {
		t.Error("a webhook without a URL should fail")
	}
}

func TestSendGotifyMessage(t *testing.T) {
	server, req, body := recordingServer(t, http.StatusOK)
	provider := &config.Config_Notification_Gotify{URL: server.URL + "/", ApiToken: "token"}

	if Err := SendGotifyMessage(testContext(), provider, "Hello", "https://img", "Title"); Err.Message != "" {
		t.Fatalf("SendGotifyMessage: %s", Err.Message)
	}
	if req.URL.Path != "/message" || req.URL.Query().Get("token") != "token" {
		t.Errorf("request = %s, want /message with the token", req.URL)
	}
	form, _ := url.ParseQuery(string(*body))
	if form.Get("message") != "Hello" || form.Get("title") != "Title" || form.Get("extras") == "" {
		t.Errorf("form = %v", form)
	}

	if Err := SendGotifyMessage(testContext(), &config.Config_Notification_Gotify{URL: server.URL}, "Hello", "", "Title"); Err.Message == "" {
		t.Error("Gotify without a token should fail")
	}
	failing, _, _ := recordingServer(t, http.StatusUnauthorized)
	if Err := SendGotifyMessage(testContext(), &config.Config_Notification_Gotify{URL: failing.URL, ApiToken: "x"}, "Hello", "", "Title"); Err.Message == "" {
		t.Error("a non-2xx response should fail")
	}
}

func TestSendDiscordMessage(t *testing.T) {
	server, _, body := recordingServer(t, http.StatusNoContent)

	if Err := SendDiscordMessage(testContext(), &config.Config_Notification_Discord{Webhook: server.URL}, "Hello", "https://img", "Title"); Err.Message != "" {
		t.Fatalf("SendDiscordMessage: %s", Err.Message)
	}
	var payload struct {
		Embeds []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Image       struct {
				URL string `json:"url"`
			} `json:"image"`
		} `json:"embeds"`
	}
	_ = json.Unmarshal(*body, &payload)
	if len(payload.Embeds) != 1 || payload.Embeds[0].Title != "Title" || payload.Embeds[0].Description != "Hello" || payload.Embeds[0].Image.URL != "https://img" {
		t.Errorf("payload = %+v", payload)
	}

	if Err := SendDiscordMessage(testContext(), &config.Config_Notification_Discord{}, "Hello", "", "Title"); Err.Message == "" {
		t.Error("Discord without a webhook should fail")
	}
}

func TestSendPushoverMessageFailsWhenTheImageCantBeDownloaded(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	// Fails before anything is sent to Pushover
	if Err := SendPushoverMessage(testContext(), &config.Config_Notification_Pushover{ApiToken: "x", UserKey: "y"}, "Hello", server.URL, "Title"); Err.Message == "" {
		t.Error("an image that can't be downloaded should fail")
	}
}
