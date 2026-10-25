package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"strconv"
	"time"
)

// OnNewItems is set at startup and called with the main media server's new items
var OnNewItems func(ctx context.Context, items []models.MediaItem)

// newItemsPageSize is how many of a library's newest items are asked for at a time
var newItemsPageSize = 50

// maxNewItems is the most new items looked at in one library
const maxNewItems = 500

// CheckForNewMediaItems adds each library's newest items until it reaches one aura already has
func CheckForNewMediaItems(ctx context.Context) (added int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Checking for New Media Items", logging.LevelInfo)
	defer logAction.Complete()

	// A full load gets the new items too
	if CurrentLibraryProgress().Running {
		logAction.AppendResult("skipped", "the libraries are loading")
		return 0, logging.LogErrorInfo{}
	}
	if cache.LibraryStore.IsEmpty() {
		logAction.AppendResult("skipped", "the libraries haven't loaded yet")
		return 0, logging.LogErrorInfo{}
	}

	server := config.Current.MediaServer
	msClient, Err := NewMediaServerClient(&server)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return 0, Err
	}

	allNewItems := []models.MediaItem{}

	for _, section := range server.Libraries {
		found, Err := msClient.GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			continue
		}

		known := cache.LibraryStore.SectionItemKeys(section.Title)

		start := 0
		for start < maxNewItems {
			items, _, fetched, Err := msClient.GetNewestLibrarySectionItems(ctx, section, strconv.Itoa(start), strconv.Itoa(newItemsPageSize))
			if Err.Message != "" {
				logAction.AppendWarning(fmt.Sprintf("%s_error", section.Title), Err.Message)
				break
			}
			if fetched == 0 {
				break
			}

			newItems := []models.MediaItem{}
			for _, item := range items {
				if !known[cache.MediaItemCacheKey(&item)] {
					newItems = append(newItems, item)
				}
			}
			if len(newItems) > 0 {
				sectionForCache := section
				sectionForCache.MediaItems = newItems
				cache.LibraryStore.UpdateSection(&sectionForCache)
				if saveErr := database.SaveLibraryItems(ctx, newItems, time.Now().Unix()); saveErr.Message != "" {
					logAction.AppendWarning("save_library_items_error", saveErr.Message)
				}
				added += len(newItems)
				allNewItems = append(allNewItems, newItems...)
				logAction.AppendResult(fmt.Sprintf("added_to_%s", section.Title), len(newItems))
			}

			// The rest are older than an item aura already has
			if len(newItems) < len(items) {
				break
			}
			start += fetched
		}
	}

	if added > 0 {
		logging.LOGGER.Info().Timestamp().Int("added", added).Msg("Added new media items from the media server")
		if OnNewItems != nil {
			OnNewItems(ctx, allNewItems)
		}
	}
	return added, logging.LogErrorInfo{}
}
