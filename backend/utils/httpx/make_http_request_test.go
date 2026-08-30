package httpx

import "testing"

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
			if got := redactURL(tt.in); got != tt.want {
				t.Errorf("redactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
