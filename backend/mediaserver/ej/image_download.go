package ej

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
)

func (e *EJ) DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Downloading and Applying %s Image for %s",
		config.Current.MediaServer.Type, utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	// Get the Image from MediUX
	// mediux.GetImage will handle checking the temp folder and caching based on config
	formatDate := imageFile.Modified.Format("20060102150405")
	imageData, _, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	// Apply the Image to the Media Item
	Err = applyImageToMediaItem(ctx, item, imageFile, imageData)
	if Err.Message != "" {
		return Err
	}

	Err = logging.LogErrorInfo{}
	return Err
}

// SaveImageAsKometaAssetOnly is a no-op on Emby/Jellyfin: AURA only writes Kometa assets
// for Plex, so there is no disk-write target to pre-stage a missing season/episode image
// to here. It logs a skip and returns without error to keep the force-preload flow silent
// on these servers.
func (e *EJ) SaveImageAsKometaAssetOnly(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Skipping Kometa asset preload for %s (not supported on %s)",
		config.Current.MediaServer.Type, utils.GetFileDownloadName(item.Title, imageFile), config.Current.MediaServer.Type,
	), logging.LevelDebug)
	defer logAction.Complete()

	logAction.AppendResult("skipped", "Kometa asset preloading is only supported on Plex")
	return logging.LogErrorInfo{}
}
