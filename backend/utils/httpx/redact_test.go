package httpx

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://gotify.example.com/message?token=abc123":                    "https://gotify.example.com/message?token=REDACTED",
		"http://plex:32400/library/sections?X-Plex-Token=secret&type=1":      "http://plex:32400/library/sections?X-Plex-Token=REDACTED&type=1",
		"https://aura.example.com/api/auth/oidc/callback?code=xyz&state=abc": "https://aura.example.com/api/auth/oidc/callback?code=REDACTED&state=REDACTED",
		"http://user:pass@host/path":                                         "http://user:REDACTED@host/path",
		"http://aura/api/images/media/item?rating_key=123&image_type=poster": "http://aura/api/images/media/item?image_type=poster&rating_key=123",
		"http://sonarr:8989/api/v3/series":                                   "http://sonarr:8989/api/v3/series",
	}
	for input, want := range tests {
		if got := RedactURL(input); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", input, got, want)
		}
	}

	err := `Get "https://gotify.example.com/message?token=abc123": dial tcp: no such host`
	if got := RedactURLInText(err, "https://gotify.example.com/message?token=abc123"); strings.Contains(got, "abc123") {
		t.Errorf("RedactURLInText left the token in %q", got)
	}

	query := RedactQuery(url.Values{"apikey": {"1"}, "page": {"2"}})
	if query.Get("apikey") != "REDACTED" || query.Get("page") != "2" {
		t.Errorf("RedactQuery = %v", query)
	}
}
