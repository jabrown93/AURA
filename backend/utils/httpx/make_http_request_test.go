package httpx

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

// net/http rewrites a password-bearing URL to "user:***@host" before storing it on the
// url.Error, so the stored URL no longer matches the raw one a substring replace looks for.
func TestRedactErrRedactsCanonicalizedURL(t *testing.T) {
	raw := "http://user:pw@host/?token=querysecret"
	err := &url.Error{
		Op:  "Get",
		URL: "http://user:***@host/?token=querysecret",
		Err: errors.New("dial tcp: connection refused"),
	}

	got := redactErr(err, raw, false)

	if strings.Contains(got, "querysecret") {
		t.Errorf("redactErr = %q, want the query credential removed", got)
	}
	if !strings.Contains(got, "connection refused") {
		t.Errorf("redactErr = %q, want the underlying cause kept", got)
	}
}

// Every component of a webhook URL has turned out to be able to carry the secret, so none
// of it is logged.
func TestRedactURLDropsWebhookURLEntirely(t *testing.T) {
	secretURLs := []string{
		"https://hooks.slack.com/services/T00000/B00000/XXXXsecretXXXX", // secret in path
		"https:supersecret",                        // secret in the opaque remainder
		"https://supersecret.hooks.example/notify", // secret in a DNS label
		"https://h/n?token=supersecret",            // secret in the query
	}
	for _, raw := range secretURLs {
		if got := redactURL(raw, true); strings.Contains(got, "secret") {
			t.Errorf("redactURL(%q, true) = %q, want no part of the URL", raw, got)
		}
	}
	// Media-server URLs name the failing endpoint and carry no secret, so they stay.
	if got := redactURL("https://plex.local/library/metadata/123", false); !strings.Contains(got, "/library/metadata/123") {
		t.Errorf("redactURL = %q, want the path kept when it is not sensitive", got)
	}
}

func TestRedactErrDropsWebhookURLEntirely(t *testing.T) {
	raw := "https://supersecret.hooks.example/services/XXXXsecretXXXX"

	// A dial failure names the host on its own, outside the URL.
	inner := errors.New("dial tcp: lookup supersecret.hooks.example: no such host")
	err := &url.Error{Op: "Post", URL: raw, Err: inner}

	got := redactErr(err, raw, true)
	if strings.Contains(got, "secret") {
		t.Errorf("redactErr = %q, want the host and path removed", got)
	}
	if !strings.Contains(got, "no such host") {
		t.Errorf("redactErr = %q, want the failure cause kept", got)
	}
}

func TestRedactErrHandlesPlainError(t *testing.T) {
	raw := "https://plex.tv/api?X-Plex-Token=abc123"
	got := redactErr(errors.New("failed calling "+raw), raw, false)

	if strings.Contains(got, "abc123") {
		t.Errorf("redactErr = %q, want the token removed", got)
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plex token param", "https://plex.tv/api/resources?X-Plex-Token=abc123", "https://plex.tv/api/resources?X-Plex-Token=%2A%2A%2A"},
		{"api key param", "https://x.dev/v1?api_key=live&page=2", "https://x.dev/v1?api_key=%2A%2A%2A&page=2"},
		{"userinfo", "https://user:pw@x.dev/v1", "https://%2A%2A%2A@x.dev/v1"},
		{"nothing sensitive", "https://x.dev/v1?page=2", "https://x.dev/v1?page=2"},
		{"unparseable", "://nope", "<unparseable url>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURL(tt.in, false); got != tt.want {
				t.Errorf("redactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
