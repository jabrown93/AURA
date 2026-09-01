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
	warmupMu   sync.Mutex
	warmupDone bool
)

func GetAllLibrarySectionsAndItems(ctx context.Context, force bool) (success bool) {
	// If we already did a run that satisfies this request, skip.
	warmupMu.Lock()
	alreadyDone := warmupDone
	warmupMu.Unlock()
	if alreadyDone && !force {
		return true
	}

	success = getAllLibrarySectionsAndItemsImpl(ctx)
	if success {
		warmupMu.Lock()
		warmupDone = true
		warmupMu.Unlock()
	}

	return success
}

func getAllLibrarySectionsAndItemsImpl(ctx context.Context) (success bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Fetching All Library Sections and Items", logging.LevelDebug)
	defer logAction.Complete()

	configuredSections := append([]models.LibrarySection(nil), config.Current.MediaServer.Libraries...)
	sort.SliceStable(configuredSections, func(i, j int) bool {
		return configuredSections[i].Title < configuredSections[j].Title
	})

	logAction.AppendResult("num_sections", len(configuredSections))

	ejRanCollections := false
	snapshots := make([]*models.LibrarySection, 0, len(configuredSections))
	for _, section := range configuredSections {
		found, Err := GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			return false
		}

		if (section.Type == "movie" || section.Type == "mixed") && !ejRanCollections {
			GetMovieCollections(ctx, section)
			if config.Current.MediaServer.Type == "Emby" || config.Current.MediaServer.Type == "Jellyfin" {
				ejRanCollections = true
			}
		}

		snapshot, ok := buildSectionSnapshot(ctx, section)
		if !ok {
			return false
		}
		snapshots = append(snapshots, snapshot)
	}

	updatedAt := time.Now().Unix()
	cache.LibraryStore.ReplaceAllSections(snapshots, updatedAt)
	cache.CollectionsStore.LastFullUpdate = updatedAt
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

// fetchAndCacheSectionItems publishes only after every page succeeds.
func fetchAndCacheSectionItems(ctx context.Context, section models.LibrarySection) bool {
	snapshot, ok := buildSectionSnapshot(ctx, section)
	if !ok {
		return false
	}
	cache.LibraryStore.UpdateSection(snapshot)
	return true
}

func buildSectionSnapshot(ctx context.Context, section models.LibrarySection) (*models.LibrarySection, bool) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return nil, false
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
	return fetchSectionSnapshot(ctx, msClient, section, states)
}

func fetchSectionSnapshot(ctx context.Context, msClient MediaServerInterface, section models.LibrarySection, states map[database.MediaItemKey]database.MediaItemState) (*models.LibrarySection, bool) {
	const pageSize = 1000
	start := 0
	expectedTotal := 0
	allItems := make([]models.MediaItem, 0)
	for {
		items, totalSize, Err := msClient.GetLibrarySectionItems(ctx, section, strconv.Itoa(start), strconv.Itoa(pageSize))
		if Err.Message != "" {
			return nil, false
		}
		tmdbIDs := make([]string, 0, len(items))
		for i := range items {
			state := states[database.MediaItemKey{TMDBID: items[i].TMDB_ID, LibraryTitle: items[i].LibraryTitle}]
			items[i].DBSavedSets = append([]models.DBSavedSet(nil), state.SavedSets...)
			items[i].IgnoredInDB = state.Ignored
			items[i].IgnoredMode = state.IgnoreMode
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
		allItems = append(allItems, items...)
		if len(items) == 0 {
			break
		}

		start += pageSize
		if expectedTotal > 0 && start >= expectedTotal {
			break
		}
		if expectedTotal == 0 && len(items) < pageSize {
			break
		}
	}
	section.MediaItems = allItems
	section.TotalSize = len(allItems)
	return &section, true
}
