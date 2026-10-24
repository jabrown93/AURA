package mediaserver

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"net/http"
)

// Fork: Kometa asset-only saves. Only Plex supports Kometa mode, so this is an optional
// interface rather than part of MediaServerInterface.
type kometaAssetSaver interface {
	SaveImageAsKometaAssetOnly(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) logging.LogErrorInfo
}

// SaveImageAsKometaAssetOnly writes an image to the Kometa asset directory without applying it
// to the server, for seasons and episodes not on the server yet. A no-op on other servers.
func SaveImageAsKometaAssetOnly(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) logging.LogErrorInfo {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	if saver, ok := msClient.(kometaAssetSaver); ok {
		return saver.SaveImageAsKometaAssetOnly(ctx, item, imageFile)
	}
	return logging.LogErrorInfo{}
}

// IsItemNotFound reports whether a media-server error was a 404, so callers can take a
// "the item is gone" path instead of leaving the operation failed for retry.
func IsItemNotFound(Err logging.LogErrorInfo) bool {
	if Err.Detail == nil {
		return false
	}
	code, ok := Err.Detail["status_code"].(int)
	return ok && code == http.StatusNotFound
}

// RefreshSectionItems picks up newly added items, e.g. a movie Radarr just imported, without
// waiting for the full library refresh. Fork: upstream's newest-items scan covers every library,
// so sectionTitle is unused.
func RefreshSectionItems(ctx context.Context, sectionTitle string) bool {
	_, Err := CheckForNewMediaItems(ctx)
	return Err.Message == ""
}
