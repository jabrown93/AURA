package plex

import (
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
