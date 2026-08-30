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

// A generic webhook is a capability URL: the secret sits in the path, not the query, so the
// whole path has to go.
func TestRedactURLHidesWebhookPath(t *testing.T) {
	raw := "https://hooks.slack.com/services/T00000/B00000/XXXXsecretXXXX"

	if got := redactURL(raw, true); strings.Contains(got, "secret") {
		t.Errorf("redactURL(hidePath) = %q, want the path removed", got)
	}
	if got := redactURL(raw, true); !strings.Contains(got, "hooks.slack.com") {
		t.Errorf("redactURL(hidePath) = %q, want the host kept for debugging", got)
	}
	// An opaque URL keeps its credential in parsed.Opaque, which String() emits while
	// ignoring Path. Validation only requires a non-empty URL, so this shape reaches here.
	if got := redactURL("https:supersecret", true); strings.Contains(got, "supersecret") {
		t.Errorf("redactURL(opaque) = %q, want the credential removed", got)
	}
	// Media-server paths name the failing endpoint and carry no secret, so they stay.
	if got := redactURL("https://plex.local/library/metadata/123", false); !strings.Contains(got, "/library/metadata/123") {
		t.Errorf("redactURL = %q, want the path kept when it is not sensitive", got)
	}
}

func TestRedactErrHidesWebhookPath(t *testing.T) {
	raw := "https://hooks.slack.com/services/T00000/B00000/XXXXsecretXXXX"
	err := &url.Error{Op: "Post", URL: raw, Err: errors.New("connection refused")}

	if got := redactErr(err, raw, true); strings.Contains(got, "secret") {
		t.Errorf("redactErr(hidePath) = %q, want the path removed", got)
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
