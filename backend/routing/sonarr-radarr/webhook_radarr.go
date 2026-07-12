package routes_sonarr_radarr

import (
	"aura/cache"
	"aura/download/auto"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type RadarrWebhookPayload struct {
	EventType string      `json:"eventType"`
	IsUpgrade bool        `json:"isUpgrade"`
	Movie     RadarrMovie `json:"movie"`
}

type RadarrMovie struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Year   int    `json:"year"`
	TmdbID int    `json:"tmdbId"`
}

// RadarrWebhookHandler handles Radarr "On Import" webhook events. When a movie is newly
// imported, it applies any saved collection sets (with Auto Download + Auto-add new
// collection items enabled) that should include the movie, immediately rather than
// waiting for the next auto-download poll. The `library` URL parameter names the Aura
// library the movie belongs to.
//
//	@Summary		Radarr Webhook
//	@Description	Apply saved collection sets to a movie newly imported by Radarr.
//	@Tags			Sonarr-Radarr
//	@Accept			json
//	@Produce		json
//	@Param			library	query		string	true	"Library title the movie belongs to"
//	@Success		200		{string}	string	"OK"
//	@Router			/api/radarr/webhook [post]
func RadarrWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Handle Radarr Webhook", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	// Get the Library from the URL params. A missing parameter is a client
	// (mis)configuration, so respond 400 rather than 500.
	libraryTitle := r.URL.Query().Get("library")
	if libraryTitle == "" {
		logAction.AppendResult("bad_request", "missing 'library' URL parameter")
		logAction.Complete()
		ld.Log()
		http.Error(w, "The 'library' URL parameter is required", http.StatusBadRequest)
		return
	}

	// Decode into typed struct
	var payload RadarrWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Only act on a new Download event. Radarr's Test/Grab/Rename/other events, and
	// upgrades of an already-imported movie (which already has artwork), are ignored.
	if payload.EventType != "Download" {
		logAction.AppendResult("event_type", payload.EventType)
		w.WriteHeader(http.StatusOK)
		return
	}
	if payload.IsUpgrade {
		logAction.AppendResult("skipped", "isUpgrade=true; existing movie already has artwork")
		w.WriteHeader(http.StatusOK)
		return
	}
	if payload.Movie.TmdbID == 0 {
		logAction.AppendResult("movie_info", "missing or invalid tmdbId")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Gate: only spin up the retrying background resolve if this library actually has a
	// saved collection set with auto-add enabled (see HasEligibleCollectionSets — not free,
	// but far cheaper than the resolve loop it guards).
	if !autodownload.HasEligibleCollectionSets(ctx, libraryTitle) {
		logAction.AppendResult("skipped", "no eligible collection sets in library")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Respond to Radarr immediately; it only cares about the status code. Continue
	// processing in the background.
	w.WriteHeader(http.StatusOK)

	tmdbID := strconv.Itoa(payload.Movie.TmdbID)
	go func(tmdbID, libraryTitle string) {
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Msgf("PANIC: in RadarrWebhookHandler background processing: %v", r)
			}
		}()

		bgCtx, bgLd := logging.CreateLoggingContext(context.Background(), "Handle Radarr Webhook Background Task")
		bgAction := bgLd.AddAction(fmt.Sprintf("Radarr Webhook: Applying collection sets for new movie (TMDB %s) in '%s'", tmdbID, libraryTitle), logging.LevelInfo)
		bgCtx = logging.WithCurrentAction(bgCtx, bgAction)

		processRadarrDownloadEvent(bgCtx, tmdbID, libraryTitle)
		bgAction.Complete()
		bgLd.Log()
	}(tmdbID, libraryTitle)
}

func processRadarrDownloadEvent(ctx context.Context, tmdbID, libraryTitle string) {
	// Give the media server time to ingest the newly imported file before we look for it.
	initialSleep := 10 * time.Second
	_, sleepAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Sleeping for %v to give the media server time to ingest the new movie", initialSleep), logging.LevelTrace)
	time.Sleep(initialSleep)
	sleepAction.Complete()

	retrySleep := 10 * time.Second
	maxRetries := 6

	var mediaItem *models.MediaItem
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(libraryTitle, tmdbID); found {
			mediaItem = item
			break
		}

		// Not in the cache yet — refresh just this section (the periodic full refresh may
		// be up to 90 minutes away) and re-check. A refresh failure is likely transient,
		// so log it and let the retry loop try again rather than bailing out.
		if ok := mediaserver.RefreshSectionItems(ctx, libraryTitle); !ok {
			logging.LOGGER.Warn().Timestamp().
				Str("tmdb_id", tmdbID).
				Str("library_title", libraryTitle).
				Int("attempt", attempt).
				Msg("Radarr Webhook: failed to refresh library section while waiting for the new movie")
		}
		if item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(libraryTitle, tmdbID); found {
			mediaItem = item
			break
		}

		if attempt == maxRetries {
			logging.LOGGER.Warn().Timestamp().
				Str("tmdb_id", tmdbID).
				Str("library_title", libraryTitle).
				Msg("Radarr Webhook: movie not found in library after retries")
			return
		}
		time.Sleep(retrySleep)
	}

	if mediaItem == nil {
		return
	}

	// Fully hydrate the item (rating key, type, tmdb id) and re-seed the cache so the
	// collection lookup can match it.
	if _, Err := mediaserver.GetMediaItemDetails(ctx, mediaItem); Err.Message != "" {
		logging.LOGGER.Warn().Timestamp().
			Str("tmdb_id", tmdbID).
			Str("library_title", libraryTitle).
			Str("error", Err.Message).
			Msg("Radarr Webhook: failed to fetch full media item details")
		return
	}

	autodownload.ApplyCollectionSetsForNewMovie(ctx, mediaItem)
}
