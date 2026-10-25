package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	otherMediaServersMu      sync.Mutex
	otherMediaServersPending atomic.Bool
)

// RefreshOtherMediaServers loads the libraries of every enabled media server that isn't the main one into its own cache.
// A media server that can't be reached is skipped with a warning.
// If a refresh is already running, it runs again once it finishes.
func RefreshOtherMediaServers() {
	otherMediaServersPending.Store(true)
	for {
		if !otherMediaServersMu.TryLock() {
			return
		}
		for otherMediaServersPending.Swap(false) {
			refreshOtherMediaServers()
		}
		otherMediaServersMu.Unlock()
		if !otherMediaServersPending.Load() {
			return
		}
	}
}

func refreshOtherMediaServers() {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Refresh Other Media Servers")
	defer ld.Log()
	logAction := ld.AddAction("Refreshing Other Media Servers", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	current := config.Current.Clone()
	var keep []string
	for _, server := range current.MediaServers {
		if server.ID == current.MainMediaServer || !server.Enabled {
			continue
		}
		keep = append(keep, server.ID)

		if connected, _ := connectMediaServer(ctx, &server); !connected {
			logAction.AppendWarning(server.Name, "Could not connect to the media server")
			logging.LOGGER.Warn().Timestamp().Str("media_server", server.Name).Str("type", server.Type).
				Msg("Could not connect to the media server, skipping it until the next refresh")
			continue
		}

		store := cache.ServerLibraryStore(server.ID)
		if !loadLibrariesIntoCache(ctx, server, store, false) {
			logAction.AppendWarning(server.Name, "Could not load the libraries")
			logging.LOGGER.Warn().Timestamp().Str("media_server", server.Name).Str("type", server.Type).
				Msg("Could not load the media server's libraries, skipping it until the next refresh")
			continue
		}
		logging.LOGGER.Info().Timestamp().Str("media_server", server.Name).
			Int("sections", store.GetSectionsCount()).Int("items", store.GetItemsCount()).
			Msg("Loaded media server sections and items into cache")
	}

	// Remove the caches of media servers that were removed or turned off
	cache.KeepServerLibraryStores(keep)
	clearOtherMediaItems()
}

// connectMediaServer checks the connection to a media server, and gets its User ID if it needs one.
// It returns the name the media server reports.
func connectMediaServer(ctx context.Context, server *config.Config_MediaServer) (connected bool, serverName string) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Connecting to Media Server: %s", server.Name), logging.LevelDebug)
	defer logAction.Complete()

	connectionOk, serverName, _, Err := TestConnection(ctx, server)
	if Err.Message != "" || !connectionOk {
		return false, ""
	}

	// Emby, Jellyfin and Silo need the User ID of an admin user
	if (server.Type == "Emby" || server.Type == "Jellyfin" || server.Type == "Silo") && server.UserID == "" {
		userID, Err := GetAdminUser(ctx, server)
		if Err.Message != "" || userID == "" {
			return false, ""
		}
		server.UserID = userID
		config.SetMediaServerUserID(server.ID, userID)
	}
	return true, serverName
}
