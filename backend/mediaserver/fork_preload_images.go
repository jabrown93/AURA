package mediaserver

import (
	"aura/logging"
	"aura/models"
	"context"
)

// ApplyImagesWithKometaPreload keeps missing season and episode targets off the media server.
func ApplyImagesWithKometaPreload(ctx, stop context.Context, item models.MediaItem, images []models.ImageFile, selected models.SelectedTypes, onUpdate func(ImageUpdate)) ([]ImageResult, models.MediaItem) {
	return applyImages(ctx, stop, item, images, func(ctx context.Context, item *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		if !models.ImageIsSelected(*item, selected, image) {
			return SaveImageAsKometaAssetOnly(ctx, item, image)
		}
		return downloadApplyImageToMediaItem(ctx, item, image)
	}, onUpdate)
}
