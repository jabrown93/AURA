package plex

import (
	"aura/cache"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"regexp"
	"strings"
)

// Legacy Plex metadata agents (e.g. HAMA for anime, and the classic
// com.plexapp.agents.* agents) only populate Plex's single primary `guid`
// string. Unlike the modern Plex agents, they do NOT populate the multi-GUID
// array returned by includeGuids=1, so AURA's normal array-based parsing finds
// nothing and the item gets dropped for lacking an ID. These helpers recover a
// provider + ID from that legacy string so such items can still be resolved.
var (
	// HAMA embeds the provider in the path: com.plexapp.agents.hama://tvdb-348545?lang=en
	hamaGuidRe = regexp.MustCompile(`^com\.plexapp\.agents\.hama://([a-zA-Z0-9]+)-([a-zA-Z0-9]+)`)
	// Classic single-provider agents: com.plexapp.agents.thetvdb://72454?lang=en
	legacyAgentGuidRe = regexp.MustCompile(`^com\.plexapp\.agents\.([a-zA-Z0-9]+)://([a-zA-Z0-9]+)`)
)

// normalizeProvider collapses the various provider spellings Plex agents emit
// into the canonical set AURA reasons about ("tmdb", "tvdb", "anidb", "imdb").
// It also folds HAMA's absolute-order variants (tvdb2/tvdb4/...) back onto the
// base provider, since the underlying series ID is the same.
func normalizeProvider(p string) string {
	p = strings.ToLower(p)
	switch {
	case strings.HasPrefix(p, "tvdb"), p == "thetvdb":
		return "tvdb"
	case strings.HasPrefix(p, "tmdb"), p == "themoviedb":
		return "tmdb"
	case strings.HasPrefix(p, "anidb"):
		return "anidb"
	case strings.HasPrefix(p, "imdb"):
		return "imdb"
	default:
		return p
	}
}

// parseLegacyGuid extracts a normalized provider and ID from a legacy Plex
// `guid` string. ok is false when the string carries no usable external ID
// (e.g. local:// unmatched items, or plex:// new-agent guids which are handled
// via the multi-GUID array instead).
func parseLegacyGuid(raw string) (provider string, id string, ok bool) {
	if raw == "" {
		return "", "", false
	}

	// HAMA: provider is encoded as a "<provider>-<id>" path segment.
	if m := hamaGuidRe.FindStringSubmatch(raw); m != nil {
		return normalizeProvider(m[1]), m[2], true
	}

	// Classic agents: the agent name is the URL scheme.
	if m := legacyAgentGuidRe.FindStringSubmatch(raw); m != nil {
		switch normalizeProvider(m[1]) {
		case "tmdb":
			return "tmdb", m[2], true
		case "tvdb":
			return "tvdb", m[2], true
		case "imdb":
			return "imdb", m[2], true
		}
	}

	return "", "", false
}

// hasResolvableGuid reports whether guids already carry an external ID (with a
// non-empty value) that resolveTMDBID can act on. Rating-only entries — which
// getGUIDsAndRatingsFromResponse produces with an empty ID for providers like
// imdb/community — don't count, so the legacy-guid fallback still fires for
// HAMA items whose only external ID lives in Plex's primary `guid` string.
func hasResolvableGuid(guids []models.MediaItemGuid) bool {
	for _, g := range guids {
		if g.ID == "" {
			continue
		}
		switch g.Provider {
		case "tmdb", "tvdb", "anidb", "imdb":
			return true
		}
	}
	return false
}

// resolveTMDBID populates item.TMDB_ID from item.Guids, applying the same
// fallback chain for both the library-list and item-details paths so anime
// (HAMA) and classic-agent items resolve consistently instead of being dropped
// in one path but not the other:
//
//  1. a direct TMDB GUID;
//  2. Plex's legacy single-`guid` string (HAMA/classic agents, which never
//     populate the multi-GUID array) — the recovered provider/id is appended to
//     item.Guids so the steps below can use it;
//  3. TVDB -> TMDB via MediUX;
//  4. AniDB -> TMDB via the Fribb mapping cache (direct TMDB id, else its TVDB
//     id through MediUX).
//
// legacyGuid is Plex's primary `guid` string (metadata.Guid); pass "" if none.
// It is a no-op once item.TMDB_ID is set.
func resolveTMDBID(ctx context.Context, item *models.MediaItem, legacyGuid string, logAction *logging.LogAction) {
	if item.TMDB_ID != "" {
		return
	}

	// 1. Direct TMDB from the GUIDs.
	for _, guid := range item.Guids {
		if guid.Provider == "tmdb" && guid.ID != "" {
			item.TMDB_ID = guid.ID
			return
		}
	}

	// 2. Legacy single-guid fallback. HAMA (anime) and the classic
	//    com.plexapp.agents.* agents populate only Plex's primary `guid`
	//    string, not the multi-GUID array, so includeGuids=1 yields nothing for
	//    them. Recover a provider/id from that string.
	if !hasResolvableGuid(item.Guids) {
		if provider, id, ok := parseLegacyGuid(legacyGuid); ok {
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: provider, ID: id})
			if provider == "tmdb" {
				item.TMDB_ID = id
				return
			}
		}
	}

	// 3. TVDB -> TMDB via MediUX.
	for _, guid := range item.Guids {
		if guid.Provider == "tvdb" {
			tmdbID, found, Err := mediux.SearchTMDBIDByTVDBID(ctx, guid.ID, item.Type)
			if Err.Message != "" {
				logAction.AppendWarning("search_tmdb_id_error", "Failed to search TMDB ID from MediUX")
			}
			if found {
				item.TMDB_ID = tmdbID
				return
			}
		}
	}

	// 4. AniDB -> TMDB via the Fribb mapping cache (Plex's HAMA agent yields
	//    AniDB IDs for anime): prefer a direct TMDB id, otherwise fall back to
	//    its TVDB id through MediUX.
	for _, guid := range item.Guids {
		if guid.Provider != "anidb" {
			continue
		}
		mapping, ok := cache.AnidbMappings.GetByAnidbID(guid.ID)
		if !ok {
			continue
		}
		if item.Type == "movie" && mapping.TMDBMovieID != "" {
			item.TMDB_ID = mapping.TMDBMovieID
			return
		}
		if item.Type != "movie" && mapping.TMDBTvID != "" {
			item.TMDB_ID = mapping.TMDBTvID
			return
		}
		if mapping.TVDBID != "" {
			tmdbID, found, Err := mediux.SearchTMDBIDByTVDBID(ctx, mapping.TVDBID, item.Type)
			if Err.Message != "" {
				logAction.AppendWarning("search_tmdb_id_error", "Failed to search TMDB ID from MediUX via AniDB TVDB fallback")
			}
			if found {
				item.TMDB_ID = tmdbID
				return
			}
		}
	}
}
