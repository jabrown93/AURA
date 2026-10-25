package ej

import (
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (e *EJ) RefreshMediaItemMetadata(ctx context.Context, item *models.MediaItem, refreshRatingKey string, updateImage bool) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Refreshing Metadata for %s - Refresh Key: %s",
		e.Config.Type,
		utils.MediaItemInfo(*item),
		refreshRatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Construct the URL for the Emby/Jellyfin API request
	u, err := url.Parse(e.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", refreshRatingKey, "Refresh")
	query := u.Query()
	query.Add("Recursive", "true")
	query.Add("ImageRefreshMode", "Default")
	query.Add("MetadataRefreshMode", "Default")
	query.Add("ReplaceAllImages", "false")
	query.Add("RegenerateTrickplay", "false")
	query.Add("ReplaceAllMetadata", "false")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Emby/Jellyfin
	resp, _, Err := makeRequest(ctx, e.Config, URL, "POST", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	return logging.LogErrorInfo{}
}
