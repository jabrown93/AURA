package plex

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver/localimage"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
)

func (p *Plex) DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Downloading and Applying %s Image for %s", utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item)),
		logging.LevelDebug)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	// Determine the Item Rating Key from Plex
	itemRatingKey := getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		message, help := utils.RatingKeyNotFoundMessage(imageFile) // Fork: name the missing season/episode
		logAction.SetError(message, help, nil)
		return *logAction.Error
	}
	logAction.AppendResult("image_rating_key", itemRatingKey)

	// If SaveImageLocally is disabled, skip downloading the image
	// Only the main media server saves images locally
	saveLocally := config.Current.Images.SaveImagesLocally.Enabled && p.Config.IsMain()
	// Fork: Kometa mode also needs the image bytes, to write them to the Kometa asset directory.
	saveKometa := config.Current.Images.Kometa.Enabled && p.Config.IsMain()
	if !saveLocally && !saveKometa {
		return applyImageToMediaItemViaMediuxURL(ctx, p.Config, item, itemRatingKey, imageFile)
	}

	// Get the Image from MediUX
	// mediux.GetImage will handle checking the temp folder and caching based on config
	formatDate := imageFile.Modified.Format("20060102150405")
	imageData, imageType, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	// Before we download and save the image locally, we need to get a list of the current posters in Plex
	// This is to handle the case where the image is already set in Plex, and we need to replace it
	// with the new image after saving it locally
	// currentImages, logErr := getAllImages(ctx, p.Config, item, itemRatingKey, "Current", imageFile.Type)
	// if logErr.Message != "" {
	// 	return logErr
	// }

	// Save the Image Locally
	if saveLocally {
		Err = localimage.SaveImageLocally(ctx, item, imageFile, imageData, imageType, p.GetMediaItemDetails)
		if Err.Message != "" {
			return Err
		}
	}

	// Fork: write the Kometa asset. Non-fatal, so a failed write never blocks applying the image.
	if saveKometa {
		if kometaErr := saveImageKometa(ctx, p, item, imageFile, imageData); kometaErr.Message != "" {
			logAction.AppendWarning("kometa_save_failed", map[string]any{"error": kometaErr.Message})
		}
	}

	// If Save Image Next to Content is enabled and the Path is set, set the poster in Plex via the MediUX URL
	// When the Path is set, the image is saved in a different location than Plex expects it to be.
	// So we need to upload the image to Plex via the MediUX URL.
	// if isCustomLocalPath {
	Err = applyImageToMediaItemViaMediuxURL(ctx, p.Config, item, itemRatingKey, imageFile)
	if Err.Message != "" {
		return Err
	}
	return Err
}

// SaveImageAsKometaAssetOnly writes the image to the Kometa asset directory without resolving a
// rating key or applying it to Plex, to pre-stage assets for seasons and episodes not on the
// server yet. Fork: Kometa mode.
func (p *Plex) SaveImageAsKometaAssetOnly(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Preloading Kometa Asset %s for %s", utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item)),
		logging.LevelDebug)
	defer logAction.Complete()

	if !config.Current.Images.Kometa.Enabled {
		logAction.AppendResult("skipped", "Kometa asset mode is disabled; nothing to preload")
		return logging.LogErrorInfo{}
	}

	formatDate := imageFile.Modified.Format("20060102150405")
	imageData, _, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}
	return saveImageKometa(ctx, p, item, imageFile, imageData)
}
