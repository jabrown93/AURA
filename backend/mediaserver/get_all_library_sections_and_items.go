package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver/plex"
	"aura/models"
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

var (
	warmupMu                         sync.Mutex
	warmupDone                       bool
	getAllLibrarySectionsAndItemsRun = getAllLibrarySectionsAndItemsImpl
)

func GetAllLibrarySectionsAndItems(ctx context.Context, force bool) (success bool) {
	// Serialize full refreshes so recovery cannot rebuild the same cache while a
	// scheduled or user-triggered refresh is already replacing it.
	warmupMu.Lock()
	defer warmupMu.Unlock()
	if warmupDone && !force {
		return true
	}

	success = getAllLibrarySectionsAndItemsRun(ctx)
	if success {
		warmupDone = true
	}
	return success
}

func getAllLibrarySectionsAndItemsImpl(ctx context.Context) (success bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Fetching All Library Sections and Items", logging.LevelDebug)
	defer logAction.Complete()

	success = true

	configuredSections := config.Current.MediaServer.Libraries

	// Sort sections by Title to ensure consistent order
	sort.SliceStable(configuredSections, func(i, j int) bool {
		return configuredSections[i].Title < configuredSections[j].Title
	})

	logAction.AppendResult("num_sections", len(configuredSections))

	ejRanCollections := false

	for _, section := range configuredSections {
		found, Err := GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			success = false
			continue
		}

		// Update the collections cache for this section
		if (section.Type == "movie" || section.Type == "mixed") && !ejRanCollections {
			GetMovieCollections(ctx, section)
			if config.Current.MediaServer.Type == "Emby" || config.Current.MediaServer.Type == "Jellyfin" {
				ejRanCollections = true
			}
		}

		if !fetchAndCacheSectionItems(ctx, section) {
			return false
		}
	}
	if !success {
		return false
	}
	cache.LibraryStore.LastFullUpdate = time.Now().Unix()
	cache.CollectionsStore.LastFullUpdate = time.Now().Unix()
	return true
}

// RefreshSectionItems re-fetches and caches the media items for a single configured
// library section, found by title. It lets callers pick up newly added items (e.g. a
// movie just imported by Radarr) without waiting for the periodic full refresh.
func RefreshSectionItems(ctx context.Context, sectionTitle string) (success bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Refreshing Library Section Items for '%s'", sectionTitle), logging.LevelDebug)
	defer logAction.Complete()

	for _, section := range config.Current.MediaServer.Libraries {
		if section.Title != sectionTitle {
			continue
		}
		found, Err := GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			return false
		}
		return fetchAndCacheSectionItems(ctx, section)
	}
	return false
}

// fetchAndCacheSectionItems pages through a library section's items and upserts them into
// the library cache. Returns false if a page fetch fails.
func fetchAndCacheSectionItems(ctx context.Context, section models.LibrarySection) bool {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return false
	}

	states, stateErr := database.GetAllMediaItemStates(ctx)
	if stateErr.Message != "" {
		logging.LOGGER.Warn().Timestamp().Str("section_title", section.Title).Str("error", stateErr.Message).Msg("Failed to bulk-fetch media item database state")
		states = map[database.MediaItemKey]database.MediaItemState{}
	}
	if plexClient, ok := msClient.(*plex.Plex); ok {
		if prepareErr := plexClient.PrepareLatestEpisodeAddedAt(ctx, section); prepareErr.Message != "" {
			logging.LOGGER.Warn().Timestamp().Str("section_title", section.Title).Str("error", prepareErr.Message).Msg("Failed to fetch latest episode dates")
		}
	}

	pageSize := 1000
	start := 0
	expectedTotal := 0
	for {
		items, totalSize, Err := msClient.GetLibrarySectionItems(ctx, section, strconv.Itoa(start), strconv.Itoa(pageSize))
		if Err.Message != "" {
			return false
		}
		tmdbIDs := make([]string, 0, len(items))
		for i := range items {
			state := states[database.MediaItemKey{TMDBID: items[i].TMDB_ID, LibraryTitle: items[i].LibraryTitle}]
			items[i].DBSavedSets = []models.DBSavedSet{}
			items[i].IgnoredInDB = state.Ignored
			items[i].IgnoredMode = state.IgnoreMode
			if !state.Ignored && len(state.SavedSets) > 0 {
				items[i].DBSavedSets = state.SavedSets
			}
			tmdbIDs = append(tmdbIDs, items[i].TMDB_ID)
		}
		if updateErr := database.UpdateMediaItemsOnServer(ctx, section.Title, tmdbIDs, true); updateErr.Message != "" {
			logging.LOGGER.Warn().Timestamp().Str("section_title", section.Title).Str("error", updateErr.Message).Msg("Failed to bulk-update media item server flags")
		}
		logging.LOGGER.Info().Timestamp().
			Str("section_title", section.Title).
			Str("section_id", section.ID).
			Int("fetched_items", len(items)).
			Int("start_index", start).
			Int("total_size", totalSize).
			Msg("Fetched library section items")

		if totalSize > 0 {
			expectedTotal = totalSize
		}
		if len(items) == 0 {
			break
		}

		sectionForCache := section
		sectionForCache.TotalSize = expectedTotal
		sectionForCache.MediaItems = items

		// Update Library Cache
		cache.LibraryStore.UpdateSection(&sectionForCache)

		start += len(items)

		if expectedTotal > 0 && start >= expectedTotal {
			break
		}
	}
	return true
}
