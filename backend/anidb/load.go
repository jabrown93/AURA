// Package anidb loads the Fribb anime-lists cross-reference dataset into the
// in-memory cache so AURA can map AniDB IDs (produced by Plex's HAMA agent for
// anime libraries) to TMDB IDs. Without it, anime items that carry only an
// AniDB ID are dropped for lacking a TMDB ID.
package anidb

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"strings"
)

// Fribb merges Anime-Lists with the manami anime-offline-database and publishes
// a single JSON array of cross-referenced IDs.
const fribbAnimeListURL = "https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json"

// fribbEntry mirrors the fields we need from an entry in anime-list-full.json.
// The ID fields are polymorphic in the dataset (number, quoted string, array,
// or a {"tv":N}/{"movie":[N]} object for themoviedb_id), so they are decoded as
// RawMessage and coerced by firstID / the themoviedb_id sub-decode below.
type fribbEntry struct {
	Type         string          `json:"type"`
	AnidbID      json.RawMessage `json:"anidb_id"`
	TvdbID       json.RawMessage `json:"tvdb_id"`
	ThemoviedbID json.RawMessage `json:"themoviedb_id"`
	ImdbID       json.RawMessage `json:"imdb_id"`
}

// PreloadAnidbMappings fetches the dataset and populates the cache. On any
// failure it degrades gracefully: it logs a warning and leaves the cache as-is,
// so AniDB-only items simply remain unresolved (the prior behavior) rather than
// failing warmup.
func PreloadAnidbMappings(ctx context.Context) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Preloading AniDB Mappings", logging.LevelInfo)
	defer logAction.Complete()

	mappings, Err := fetchAnidbMappings(ctx)
	if Err.Message != "" {
		logAction.AppendWarning("anidb_mapping_error", Err.Message)
		return
	}

	cache.AnidbMappings.StoreAnidbMappings(mappings)
	logAction.AppendResult("mappings_count", len(mappings))
	logging.LOGGER.Info().Timestamp().Int("mappings", cache.AnidbMappings.Count()).Msg("Loaded AniDB mappings into cache")
}

// fribbFetchTimeoutSeconds bounds the whole request (connect + ~7.5MB body
// read). The preload runs synchronously on the warmup critical path, so this is
// deliberately far below a default so an unreachable/blocked host (e.g. GitHub
// down, or egress routed through a misconfigured VPN) can't stall Plex startup;
// on timeout PreloadAnidbMappings degrades gracefully and the weekly cron / next
// warmup retries. It stays generous enough to download the dataset over a slow
// link (~250 KB/s completes well within the window).
const fribbFetchTimeoutSeconds = 30

func fetchAnidbMappings(ctx context.Context) ([]models.AnidbMapping, logging.LogErrorInfo) {
	_, body, Err := httpx.MakeHTTPRequest(ctx, fribbAnimeListURL, "GET", nil, fribbFetchTimeoutSeconds, nil, "Fribb AniDB Mappings")
	if Err.Message != "" {
		return nil, Err
	}

	var entries []fribbEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, logging.LogErrorInfo{
			Message: "Failed to parse AniDB mappings dataset",
			Help:    "The Fribb anime-lists JSON could not be decoded",
			Detail:  map[string]any{"error": err.Error()},
		}
	}

	out := make([]models.AnidbMapping, 0, len(entries))
	for _, e := range entries {
		anidbID := firstID(e.AnidbID)
		if anidbID == "" {
			continue
		}

		tmdbTv, tmdbMovie := parseThemoviedbID(e.ThemoviedbID)
		m := models.AnidbMapping{
			AnidbID:     anidbID,
			Type:        e.Type,
			TMDBTvID:    tmdbTv,
			TMDBMovieID: tmdbMovie,
			TVDBID:      firstID(e.TvdbID),
			IMDBID:      firstID(e.ImdbID),
		}

		// Keep only entries that carry at least one ID AURA can resolve to TMDB.
		if m.TMDBTvID == "" && m.TMDBMovieID == "" && m.TVDBID == "" {
			continue
		}
		out = append(out, m)
	}

	return out, logging.LogErrorInfo{}
}

// parseThemoviedbID decodes the {"tv":N} / {"movie":[N]} object form.
func parseThemoviedbID(raw json.RawMessage) (tv string, movie string) {
	if isEmptyRaw(raw) {
		return "", ""
	}
	var tf struct {
		TV    json.RawMessage `json:"tv"`
		Movie json.RawMessage `json:"movie"`
	}
	if err := json.Unmarshal(raw, &tf); err != nil {
		return "", ""
	}
	return firstID(tf.TV), firstID(tf.Movie)
}

// firstID coerces a polymorphic JSON id value (number, quoted string, or array
// of either, possibly nested) into a single plain string. Returns "" for null,
// empty, or unparseable input.
func firstID(raw json.RawMessage) string {
	if isEmptyRaw(raw) {
		return ""
	}
	s := strings.TrimSpace(string(raw))
	if strings.HasPrefix(s, "[") {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
			return ""
		}
		return firstID(arr[0])
	}
	return strings.Trim(s, `"`)
}

func isEmptyRaw(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}
