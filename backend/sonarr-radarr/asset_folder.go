package sonarr_radarr

import (
	"aura/config"
	"aura/logging"
	"context"
	"path"
	"strconv"
	"strings"
)

// GetItemFolderName resolves the on-disk folder name that Sonarr/Radarr manages for the item
// with the given TMDB ID in the given media-server library. This is the leaf directory name
// (e.g. "The Last of Us (2023)"), which is exactly the per-item asset folder Kometa expects, so
// it can substitute for a Plex file-path lookup when the media server can no longer resolve the
// item (e.g. Plex returns a 404 for a stale rating key).
//
// Sonarr/Radarr owns the same physical folder the media server scans, so its reported Path is
// authoritative for the folder name even when the media server has lost the item. Only the leaf
// name matters for a Kometa asset folder, so mount-prefix differences between Sonarr/Radarr and
// AURA are irrelevant.
//
// Return semantics:
//   - found=true: folderName is usable.
//   - found=false, Err empty: nothing to fall back to — Sonarr/Radarr is not configured, no app
//     matches the item's type+library, or the only matching app has incomplete config (it is
//     skipped). The caller should keep its normal failure behavior.
//   - found=false, Err set: a matching, fully-configured app was found but the item lookup or the
//     path derivation failed.
func GetItemFolderName(ctx context.Context, tmdbID, itemType, libraryTitle string) (folderName string, found bool, Err logging.LogErrorInfo) {
	if len(config.Current.SonarrRadarr.Applications) == 0 {
		return "", false, logging.LogErrorInfo{}
	}

	// Movies live in Radarr, shows in Sonarr.
	var wantType string
	switch itemType {
	case "movie":
		wantType = "Radarr"
	case "show":
		wantType = "Sonarr"
	default:
		return "", false, logging.LogErrorInfo{}
	}

	tmdbIDInt, convErr := strconv.Atoi(tmdbID)
	if convErr != nil || tmdbIDInt == 0 {
		return "", false, logging.LogErrorInfo{}
	}

	ctx, logAction := logging.AddSubActionToContext(ctx, "Resolving Sonarr/Radarr asset folder name", logging.LevelDebug)
	defer logAction.Complete()

	for _, app := range config.Current.SonarrRadarr.Applications {
		// Match the same way HandleTags does: the app type must match the item type, and the
		// app's library must be the item's library (a single TMDB ID can exist in several).
		if app.Type != wantType || app.Library != libraryTitle {
			continue
		}
		if Err = MakeSureAllAppInfoPresent(ctx, &app); Err.Message != "" {
			continue
		}

		srItem, itemErr := GetItemInfoFromTMDBID(ctx, app, tmdbIDInt)
		if itemErr.Message != "" {
			// Matching app configured, but the item is not present there (or it is unreachable).
			return "", false, itemErr
		}

		diskPath := itemPath(srItem)
		if diskPath == "" {
			return "", false, logging.LogErrorInfo{
				Message: "Sonarr/Radarr returned no path for the item",
				Help:    "Ensure the item has a root folder/path configured in Sonarr/Radarr",
				Detail:  map[string]any{"tmdb_id": tmdbID, "library": libraryTitle, "type": itemType},
			}
		}

		folderName = folderNameFromPath(diskPath)
		if folderName == "" {
			return "", false, logging.LogErrorInfo{
				Message: "Could not derive an asset folder name from the Sonarr/Radarr path",
				Help:    "Check the item's path in Sonarr/Radarr",
				Detail:  map[string]any{"path": diskPath},
			}
		}
		logAction.AppendResult("asset_folder", folderName)
		return folderName, true, logging.LogErrorInfo{}
	}

	// No configured app matched this item's type+library.
	return "", false, logging.LogErrorInfo{}
}

// itemPath extracts the item's on-disk folder path from a Sonarr/Radarr item response. Both item
// types embed SR_ItemInfoBase, whose Path is the full path to the item's own folder on disk.
func itemPath(srItem any) string {
	switch v := srItem.(type) {
	case SR_SonarrItem:
		return v.Path
	case SR_RadarrItem:
		return v.Path
	default:
		return ""
	}
}

// folderNameFromPath returns the leaf directory name of a path, tolerating both Windows and POSIX
// separators (Sonarr/Radarr may run on Windows and report backslash paths).
func folderNameFromPath(p string) string {
	normalized := strings.ReplaceAll(p, "\\", "/")
	normalized = strings.TrimRight(normalized, "/")
	base := path.Base(normalized)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return base
}
