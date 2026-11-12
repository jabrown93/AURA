package sonarr_radarr

import "testing"

// TestItemPath keeps Kometa fallback working with upstream's lossless SR_Item model.
func TestItemPath(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"movie", `{"id":1,"path":"/movies/Movie (2026)"}`, "/movies/Movie (2026)"},
		{"show", `{"id":2,"path":"C:\\TV\\Show"}`, `C:\TV\Show`},
		{"missing", `{"id":3}`, ""},
		{"wrong type", `{"id":4,"path":42}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := parseSRItem([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if got := itemPath(item); got != tc.want {
				t.Fatalf("itemPath = %q, want %q", got, tc.want)
			}
		})
	}
}
