package plex

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
)

func TestHasResolvableGuid(t *testing.T) {
	cases := []struct {
		name  string
		guids []models.MediaItemGuid
		want  bool
	}{
		{"empty", nil, false},
		{"rating-only imdb (no id)", []models.MediaItemGuid{{Provider: "imdb", Rating: "7.9"}}, false},
		{"rating-only community", []models.MediaItemGuid{{Provider: "community", Rating: "8.1"}}, false},
		{"real tmdb", []models.MediaItemGuid{{Provider: "tmdb", ID: "123"}}, true},
		{"real tvdb", []models.MediaItemGuid{{Provider: "tvdb", ID: "456"}}, true},
		{"real anidb", []models.MediaItemGuid{{Provider: "anidb", ID: "789"}}, true},
		{"real imdb", []models.MediaItemGuid{{Provider: "imdb", ID: "tt42"}}, true},
		{"unknown provider with id", []models.MediaItemGuid{{Provider: "plex", ID: "abc"}}, false},
		{"rating imdb + real tvdb", []models.MediaItemGuid{{Provider: "imdb", Rating: "7.9"}, {Provider: "tvdb", ID: "456"}}, true},
	}
	for _, tc := range cases {
		if got := hasResolvableGuid(tc.guids); got != tc.want {
			t.Errorf("hasResolvableGuid(%s) = %v; want %v", tc.name, got, tc.want)
		}
	}
}

// TestResolveTMDBID covers the branches that resolve without touching MediUX or
// the AniDB cache (direct TMDB, legacy-guid fallback, and graceful
// non-resolution). TVDB->MediUX and AniDB-cache-hit paths call external
// dependencies and are exercised against the live instance, not here.
func TestResolveTMDBID(t *testing.T) {
	la := logging.NewLogData("test").AddAction("test", logging.LevelInfo)

	t.Run("no-op when already set", func(t *testing.T) {
		item := &models.MediaItem{TMDB_ID: "999", Type: "show",
			Guids: []models.MediaItemGuid{{Provider: "tvdb", ID: "456"}}}
		resolveTMDBID(context.Background(), item, "", la)
		if item.TMDB_ID != "999" {
			t.Errorf("TMDB_ID = %q; want unchanged 999", item.TMDB_ID)
		}
	})

	t.Run("direct tmdb guid", func(t *testing.T) {
		item := &models.MediaItem{Type: "movie",
			Guids: []models.MediaItemGuid{{Provider: "tmdb", ID: "555"}}}
		resolveTMDBID(context.Background(), item, "", la)
		if item.TMDB_ID != "555" {
			t.Errorf("TMDB_ID = %q; want 555", item.TMDB_ID)
		}
	})

	t.Run("legacy HAMA tmdb string, empty guids", func(t *testing.T) {
		item := &models.MediaItem{Type: "show"}
		resolveTMDBID(context.Background(), item, "com.plexapp.agents.hama://tmdb-777?lang=en", la)
		if item.TMDB_ID != "777" {
			t.Errorf("TMDB_ID = %q; want 777", item.TMDB_ID)
		}
		// The recovered provider/id should have been appended for downstream use.
		if len(item.Guids) != 1 || item.Guids[0].Provider != "tmdb" || item.Guids[0].ID != "777" {
			t.Errorf("appended guids = %+v; want one tmdb/777", item.Guids)
		}
	})

	t.Run("legacy classic themoviedb string", func(t *testing.T) {
		item := &models.MediaItem{Type: "movie"}
		resolveTMDBID(context.Background(), item, "com.plexapp.agents.themoviedb://888?lang=en", la)
		if item.TMDB_ID != "888" {
			t.Errorf("TMDB_ID = %q; want 888", item.TMDB_ID)
		}
	})

	t.Run("rating-only imdb guid does not block legacy fallback", func(t *testing.T) {
		// Mirrors the details path: getGUIDsAndRatingsFromResponse yields a
		// rating-only imdb entry (no ID) for a HAMA item whose real id lives in
		// the legacy guid string. hasResolvableGuid must ignore it so the
		// fallback still fires.
		item := &models.MediaItem{Type: "show",
			Guids: []models.MediaItemGuid{{Provider: "imdb", Rating: "7.9"}}}
		resolveTMDBID(context.Background(), item, "com.plexapp.agents.hama://tmdb-321?lang=en", la)
		if item.TMDB_ID != "321" {
			t.Errorf("TMDB_ID = %q; want 321", item.TMDB_ID)
		}
	})

	t.Run("no external id and empty legacy guid", func(t *testing.T) {
		item := &models.MediaItem{Type: "show"}
		resolveTMDBID(context.Background(), item, "", la)
		if item.TMDB_ID != "" {
			t.Errorf("TMDB_ID = %q; want empty", item.TMDB_ID)
		}
	})

	t.Run("anidb with empty cache resolves to nothing", func(t *testing.T) {
		// With no mappings preloaded, the AniDB lookup misses and the item is
		// left unresolved rather than erroring.
		item := &models.MediaItem{Type: "show",
			Guids: []models.MediaItemGuid{{Provider: "anidb", ID: "12345"}}}
		resolveTMDBID(context.Background(), item, "", la)
		if item.TMDB_ID != "" {
			t.Errorf("TMDB_ID = %q; want empty (cache miss)", item.TMDB_ID)
		}
	})
}
