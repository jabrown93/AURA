package plex

import "testing"

func TestParseLegacyGuid(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		wantProvider string
		wantID       string
		wantOK       bool
	}{
		{"hama tvdb", "com.plexapp.agents.hama://tvdb-348545?lang=en", "tvdb", "348545", true},
		{"hama anidb", "com.plexapp.agents.hama://anidb-15275?lang=en", "anidb", "15275", true},
		{"hama tmdb", "com.plexapp.agents.hama://tmdb-95479?lang=en", "tmdb", "95479", true},
		{"hama imdb", "com.plexapp.agents.hama://imdb-tt12343534?lang=en", "imdb", "tt12343534", true},
		{"hama tvdb absolute variant", "com.plexapp.agents.hama://tvdb2-84911?lang=en", "tvdb", "84911", true},
		{"hama no lang suffix", "com.plexapp.agents.hama://tvdb-348545", "tvdb", "348545", true},
		{"classic themoviedb", "com.plexapp.agents.themoviedb://85937?lang=en", "tmdb", "85937", true},
		{"classic thetvdb", "com.plexapp.agents.thetvdb://72454", "tvdb", "72454", true},
		{"classic imdb", "com.plexapp.agents.imdb://tt0119698?lang=en", "imdb", "tt0119698", true},
		{"local unmatched", "local://33697", "", "", false},
		{"new agent plex guid", "plex://show/5d9c081be98e47001eb0d74f", "", "", false},
		{"empty", "", "", "", false},
		{"unknown agent", "com.plexapp.agents.none://-", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, id, ok := parseLegacyGuid(tc.raw)
			if provider != tc.wantProvider || id != tc.wantID || ok != tc.wantOK {
				t.Fatalf("parseLegacyGuid(%q) = (%q, %q, %v); want (%q, %q, %v)",
					tc.raw, provider, id, ok, tc.wantProvider, tc.wantID, tc.wantOK)
			}
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	cases := map[string]string{
		"tmdb":       "tmdb",
		"themoviedb": "tmdb",
		"tvdb":       "tvdb",
		"tvdb2":      "tvdb",
		"tvdb4":      "tvdb",
		"anidb":      "anidb",
		"imdb":       "imdb",
		"IMDB":       "imdb",
		"anilist":    "anilist",
	}
	for in, want := range cases {
		if got := normalizeProvider(in); got != want {
			t.Errorf("normalizeProvider(%q) = %q; want %q", in, got, want)
		}
	}
}
