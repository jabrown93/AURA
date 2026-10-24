package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Items found on the other media servers are kept for a short time, as the images of a set are applied one at a time
const otherMediaItemMaxAge = 10 * time.Minute

type otherMediaItemKey struct {
	serverID, itemType, tmdbID, libraryTitle, edition string
}

type otherMediaItem struct {
	item    models.MediaItem
	found   bool
	addedAt time.Time
}

var (
	otherMediaItemsMu sync.Mutex
	otherMediaItems   = map[otherMediaItemKey]otherMediaItem{}
)

// Creates the client for a media server that isn't the main one. Replaced in tests.
var newOtherMediaServerClient = NewMediaServerClient

// clearOtherMediaItems forgets the items found on the other media servers
func clearOtherMediaItems() {
	otherMediaItemsMu.Lock()
	defer otherMediaItemsMu.Unlock()
	otherMediaItems = map[otherMediaItemKey]otherMediaItem{}
}

// otherMediaServers returns the enabled media servers that aren't the main one
func otherMediaServers(supported func(server config.Config_MediaServer) bool) []config.Config_MediaServer {
	current := config.Current.Clone()
	var servers []config.Config_MediaServer
	for _, server := range current.MediaServers {
		if server.ID == current.MainMediaServer || !server.Enabled || (supported != nil && !supported(server)) {
			continue
		}
		servers = append(servers, server)
	}
	return servers
}

// mappedLibraries returns the libraries of a media server that match a main server library
func mappedLibraries(server config.Config_MediaServer, mainLibrary string) []string {
	// An empty value means a library doesn't match any main server library
	if mainLibrary == "" {
		return nil
	}
	var libraries []string
	for library, matches := range server.LibraryMap {
		if matches == mainLibrary {
			libraries = append(libraries, library)
		}
	}
	slices.Sort(libraries)
	return libraries
}

// findInLibraryStore finds a main media server item in a library cache, in the given libraries.
// The same edition is used first, then the base item.
func findInLibraryStore(store *cache.MediaServerLibraryCache, libraries []string, mainItem models.MediaItem) (item models.MediaItem, library string, found bool) {
	editions := []string{mainItem.Edition}
	if mainItem.Edition != "" {
		editions = append(editions, "")
	}
	for _, edition := range editions {
		for _, library := range libraries {
			cached, ok := store.GetMediaItemFromSectionByTMDBIDAndEdition(library, mainItem.TMDB_ID, edition)
			if ok && cached.Type == mainItem.Type {
				return *cached, library, true
			}
		}
	}
	return models.MediaItem{}, "", false
}

// FindMediaItemOnMediaServer finds an item of the main media server on another media server, with its full details.
// Only the libraries that match the item's library are searched, for the same edition first and then the base item.
func FindMediaItemOnMediaServer(ctx context.Context, server config.Config_MediaServer, mainItem models.MediaItem) (item models.MediaItem, found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Finding %s on Media Server: %s", utils.MediaItemInfo(mainItem), server.Name), logging.LevelDebug)
	defer logAction.Complete()

	key := otherMediaItemKey{server.ID, mainItem.Type, mainItem.TMDB_ID, mainItem.LibraryTitle, mainItem.Edition}
	otherMediaItemsMu.Lock()
	saved, ok := otherMediaItems[key]
	otherMediaItemsMu.Unlock()
	if ok && time.Since(saved.addedAt) < otherMediaItemMaxAge {
		return saved.item, saved.found, logging.LogErrorInfo{}
	}

	// Search the library cache of the media server
	item, _, found = findInLibraryStore(cache.ServerLibraryStore(server.ID), mappedLibraries(server, mainItem.LibraryTitle), mainItem)

	// Get the full details, which include the seasons and episodes
	if found {
		msClient, Err := newOtherMediaServerClient(&server)
		if Err.Message != "" {
			return item, false, Err
		}
		detailsFound, Err := msClient.GetMediaItemDetails(ctx, &item)
		if Err.Message != "" {
			return item, false, Err
		}
		found = detailsFound
	}
	logAction.AppendResult("found", found)

	otherMediaItemsMu.Lock()
	otherMediaItems[key] = otherMediaItem{item: item, found: found, addedAt: time.Now()}
	otherMediaItemsMu.Unlock()
	return item, found, logging.LogErrorInfo{}
}

// applyToOtherMediaServers runs apply for the item on every other media server that has it.
// It is logged on its own, so problems on another media server are warnings and don't change the result for the main one.
func applyToOtherMediaServers(mainItem models.MediaItem, what string, supported func(server config.Config_MediaServer) bool,
	apply func(ctx context.Context, msClient MediaServerInterface, item *models.MediaItem) logging.LogErrorInfo) {
	servers := otherMediaServers(supported)
	if len(servers) == 0 {
		return
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "Apply to Other Media Servers")
	defer ld.Log()
	logAction := ld.AddAction(fmt.Sprintf("Applying %s to Other Media Servers for %s", what, utils.MediaItemInfo(mainItem)), logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	for _, server := range servers {
		// Emby, Jellyfin and Silo need the User ID, which is set once aura has connected to the media server.
		// The library refresh already warns when it can't connect.
		if (server.Type == "Emby" || server.Type == "Jellyfin" || server.Type == "Silo") && server.UserID == "" {
			logAction.AppendResult(server.Name, "not connected")
			continue
		}

		item, found, Err := FindMediaItemOnMediaServer(ctx, server, mainItem)
		if Err.Message != "" {
			warnOtherMediaServer(logAction, server, mainItem, what, Err.Message)
			continue
		}
		if !found {
			logAction.AppendResult(server.Name, "not found")
			continue
		}

		msClient, Err := newOtherMediaServerClient(&server)
		if Err.Message == "" {
			Err = apply(ctx, msClient, &item)
		}
		if Err.Message != "" {
			warnOtherMediaServer(logAction, server, mainItem, what, Err.Message)
			continue
		}
		logAction.AppendResult(server.Name, "applied")
	}
}

func warnOtherMediaServer(logAction *logging.LogAction, server config.Config_MediaServer, item models.MediaItem, what, message string) {
	logAction.AppendWarning(server.Name, message)
	logging.LOGGER.Warn().Timestamp().Str("media_server", server.Name).Str("item", utils.MediaItemInfo(item)).Str("error", message).
		Msgf("Could not apply %s to the media server", what)
}
