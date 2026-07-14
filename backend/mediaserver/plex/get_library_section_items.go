package plex

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

func (p *Plex) GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Fetching Items for Library Section: %s", section.Title,
	), logging.LevelInfo)
	defer logAction.Complete()

	items = []models.MediaItem{}
	totalSize = 0
	Err = logging.LogErrorInfo{}

	// If limit is empty, set a default limit
	if limit == "" {
		limit = "1000"
	}

	// Construct the URL for the Plex library sections API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return items, totalSize, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", section.ID, "all")
	query := u.Query()
	query.Set("X-Plex-Container-Start", sectionStartIndex)
	query.Set("X-Plex-Container-Size", limit)
	query.Set("includeGuids", "1")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return items, totalSize, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Library Items Response")
	if Err.Message != "" {
		return items, totalSize, *logAction.Error
	}

	totalSize = plexResp.MediaContainer.TotalSize

	for _, metadata := range plexResp.MediaContainer.Metadata {
		var item models.MediaItem
		item.RatingKey = metadata.RatingKey
		item.Type = metadata.Type
		item.Title = metadata.Title
		item.Year = metadata.Year
		item.LibraryTitle = plexResp.MediaContainer.LibrarySectionTitle
		item.UpdatedAt = metadata.UpdatedAt
		item.AddedAt = metadata.AddedAt
		item.ContentRating = metadata.ContentRating
		item.Summary = metadata.Summary

		if item.Title == "" {
			if metadata.OriginalTitle != "" {
				item.Title = metadata.OriginalTitle
			} else {
				item.Title = "<Unknown Title>"
			}
		}

		if t, err := time.Parse("2006-01-02", metadata.OriginallyAvailableAt); err == nil {
			item.ReleasedAt = t.Unix()
		} else {
			item.ReleasedAt = 0
		}

		if metadata.Type == "movie" {
			item.Movie = &models.MediaItemMovie{
				File: models.MediaItemFile{
					Path:     metadata.Media[0].Part[0].File,
					Size:     metadata.Media[0].Part[0].Size,
					Duration: metadata.Media[0].Part[0].Duration,
				},
			}
		}

		// Parse the modern multi-GUID array (populated by the new Plex agents
		// when includeGuids=1 is requested).
		for _, guid := range metadata.Guids {
			if guid.ID == "" {
				continue
			}
			// Sample guid.id : tmdb://######
			// Split into provider and id
			parts := strings.SplitN(guid.ID, "://", 2)
			if len(parts) != 2 {
				continue
			}
			item.Guids = append(item.Guids, models.MediaItemGuid{
				Provider: normalizeProvider(parts[0]),
				ID:       parts[1],
			})
		}

		// Resolve the TMDB ID from the GUIDs, falling back to Plex's legacy
		// single-guid string (HAMA/classic agents, which never populate the
		// multi-GUID array) and the Fribb AniDB mapping so anime and
		// classic-agent items resolve instead of being dropped.
		resolveTMDBID(ctx, &item, metadata.Guid, logAction)

		if item.TMDB_ID == "" {
			logging.LOGGER.Warn().Timestamp().Str("item_title", item.Title).Str("library_section", section.Title).Msgf("Skipping item in '%s' as no TMDB ID could be found", section.Title)
			totalSize-- // Decrement total size as this item will be skipped
			continue    // Skip items without TMDB ID
		}

		// Check if Media Item exists in DB
		ignored, ignoredMode, sets, logErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle)
		if logErr.Message != "" {
			logAction.AppendWarning("message", "Failed to check if media item exists in database")
			logAction.AppendWarning("error", Err)
		}
		if !ignored {
			item.DBSavedSets = sets
		} else {
			item.IgnoredInDB = true
			item.IgnoredMode = ignoredMode
		}

		// Update the Media Item on Server in the DB
		updateErr := database.UpdateMediaItemOnServer(ctx, item.TMDB_ID, item.LibraryTitle, true)
		if updateErr.Message != "" {
			logAction.AppendWarning("update_on_server_error", updateErr.Message)
		}

		// Check if Media Item exists in MediUX with a set
		if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
			item.HasMediuxSets = true
		}

		// If the item is a movie, update the movie collections cache
		if item.Type == "movie" {
			if len(metadata.Collections) > 0 {
				for _, coll := range metadata.Collections {
					cache.CollectionsStore.UpdateMediaItemInCollectionByTitle(coll.Tag, &item)
				}
			}
		}

		cache.LibraryStore.UpdateMediaItem(section.Title, &item)
		items = append(items, item)
	}

	// For show sections, bulk-fetch all episodes to compute LatestEpisodeAddedAt per show.
	if section.Type == "show" && config.Current.MediaServer.EnableSortByEpisodeAddedDate {
		latestEpAdded, fetchErr := fetchLatestEpisodeAddedAtByShow(ctx, section.ID)
		if fetchErr.Message != "" {
			logAction.AppendWarning("latest_episode_added_at", "Failed to bulk-fetch latest episode addedAt for shows")
		} else {
			for i := range items {
				items[i].LatestEpisodeAddedAt = latestEpAdded[items[i].RatingKey]
			}
		}
	}

	return items, totalSize, logging.LogErrorInfo{}
}

// fetchLatestEpisodeAddedAtByShow fetches all episodes for a library section in one bulk
// request and returns a map of show RatingKey -> latest episode addedAt timestamp.
func fetchLatestEpisodeAddedAtByShow(ctx context.Context, sectionID string) (map[string]int64, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Bulk-fetching episode addedAt for section %s", sectionID,
	), logging.LevelDebug)
	defer logAction.Complete()

	logging.DevMsgf("Bulk-fetching latest episode addedAt for shows in section %s", sectionID)

	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return nil, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", sectionID, "all")

	// First pass: get total episode count (size=0 returns totalSize without data)
	query := u.Query()
	query.Set("type", "4") // 4 = episode
	query.Set("X-Plex-Container-Start", "0")
	query.Set("X-Plex-Container-Size", "0")
	u.RawQuery = query.Encode()

	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var countResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &countResp, "Plex Episode Count Response")
	if Err.Message != "" {
		return nil, *logAction.Error
	}
	totalEpisodes := countResp.MediaContainer.TotalSize
	if totalEpisodes == 0 {
		return map[string]int64{}, logging.LogErrorInfo{}
	}

	// Second pass: fetch all episodes in one shot
	query.Set("X-Plex-Container-Size", fmt.Sprintf("%d", totalEpisodes))
	u.RawQuery = query.Encode()

	resp, respBody, Err = makeRequest(ctx, config.Current.MediaServer, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var episodesResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &episodesResp, "Plex All Episodes Response")
	if Err.Message != "" {
		return nil, *logAction.Error
	}

	latest := make(map[string]int64)
	for _, ep := range episodesResp.MediaContainer.Metadata {
		showKey := ep.GrandParentRatingKey
		if ep.AddedAt > latest[showKey] {
			latest[showKey] = ep.AddedAt
		}
	}

	return latest, logging.LogErrorInfo{}
}
