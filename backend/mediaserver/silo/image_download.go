package silo

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

func (s *Silo) DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Downloading and Applying %s Image for %s", utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item)),
		logging.LevelDebug)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	// Determine the Item Rating Key from Silo
	itemRatingKey := getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		logAction.SetError("Failed to determine Rating Key for Media Item", "Ensure the Media Item and Image File data are correct", nil)
		return *logAction.Error
	}
	logAction.AppendResult("image_rating_key", itemRatingKey)

	// If SaveImageLocally is enabled, save the image next to the content as well
	// Silo does not use the saved image, so the image is still applied using the MediUX URL
	// Only the main media server saves images locally
	if config.Current.Images.SaveImagesLocally.Enabled && s.Config.IsMain() {
		// Get the Image from MediUX
		// mediux.GetImage will handle checking the temp folder and caching based on config
		formatDate := imageFile.Modified.Format("20060102150405")
		imageData, imageType, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
		if Err.Message != "" {
			return Err
		}

		// Save the Image Locally
		Err = localimage.SaveImageLocally(ctx, item, imageFile, imageData, imageType, s.GetMediaItemDetails)
		if Err.Message != "" {
			return Err
		}
	}

	// With the aura plugin, a refresh gets the saved set's images from the plugin, so the item's images stay unlocked
	if auraPluginActive(ctx, s, item.LibraryTitle) {
		scheduleUnlockAndRefresh(s, *item)
		logAction.AppendResult("aura_plugin", "Silo will refresh the item to get the image from the aura plugin")
		return logging.LogErrorInfo{}
	}

	// Construct the MediUX Image URL
	// Silo downloads the image from MediUX itself
	imageURL, Err := mediux.ConstructImageUrl(ctx, imageFile.ID, imageFile.Modified.String(), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// Apply the Image to the Media Item
	Err = applyImageToMediaItem(ctx, s.Config, item, itemRatingKey, imageURL, imageFile.Type)
	if Err.Message == "" {
		return logging.LogErrorInfo{}
	}
	if !isNotFoundError(Err) {
		return Err
	}

	// If the item was not found, Silo may have changed its Content ID
	// Get the latest Media Item details, which finds the new Content ID, and try again
	logAction.AppendWarning("message", "Media item not found on Silo server, getting the latest details and trying again")
	found, Err := s.GetMediaItemDetails(ctx, item)
	if Err.Message != "" || !found {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	itemRatingKey = getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		logAction.SetError("Failed to determine Rating Key for Media Item", "Ensure the Media Item and Image File data are correct", nil)
		return *logAction.Error
	}
	logAction.AppendResult("new_image_rating_key", itemRatingKey)

	return applyImageToMediaItem(ctx, s.Config, item, itemRatingKey, imageURL, imageFile.Type)
}
