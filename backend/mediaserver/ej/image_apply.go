package ej

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
)

func applyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile, imageData []byte) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Applying %s Image for %s",
		config.Current.MediaServer.Type, utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Determine the Item Rating Key from Emby/Jellyfin
	itemRatingKey := getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		message, help := utils.RatingKeyNotFoundMessage(imageFile)
		logAction.SetError(message, help, nil)
		return *logAction.Error
	}

	// Handling for Backdrops are different than Primary Images
	if imageFile.Type != "backdrop" {
		// Apply the Image to the Media Item
		Err = uploadImage(ctx, item, itemRatingKey, imageFile, imageData)
		if Err.Message != "" {
			return Err
		}
	} else {
		// For Backdrops, we need to set the index of the new upload to 0
		// To do this, we get a list of current images
		// Then we upload the new image
		// Then we get a list of current images again and find the new image's ID
		// Then we set the index of that image to 0

		// Get current images
		currentImages, Err := getCurrentImages(ctx, item, "Current")
		if Err.Message != "" {
			return *logAction.Error
		}

		// Upload the new image
		Err = uploadImage(ctx, item, itemRatingKey, imageFile, imageData)
		if Err.Message != "" {
			return *logAction.Error
		}

		if len(currentImages) != 0 {
			// Get images after upload
			updatedImages, Err := getCurrentImages(ctx, item, "Updated")
			if Err.Message != "" {
				return *logAction.Error
			}

			// Find the new image's ID
			newImage := findNewImage(ctx, currentImages, updatedImages)
			if newImage.ImageTag == "" && newImage.ImagePath == "" {
				logAction.SetError("Failed to find newly uploaded Backdrop image", "Ensure the image upload was successful", map[string]any{
					"item":           *item,
					"image_file":     imageFile,
					"current_images": currentImages,
					"updated_images": updatedImages,
				})
				return *logAction.Error
			}

			// Now we change the image index to 0, if it's not already 0
			if newImage.ImageIndex != 0 {
				err := updateImageIndex(ctx, item, newImage)
				if err.Message != "" {
					return *logAction.Error
				}
			}
		}
	}

	return Err
}
