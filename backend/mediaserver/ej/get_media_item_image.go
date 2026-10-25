package ej

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (e *EJ) GetMediaItemImage(ctx context.Context, item *models.MediaItem, imageRatingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Image (%s) for %s",
		e.Config.Type,
		imageType,
		utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	imageData = []byte{}
	Err = logging.LogErrorInfo{}

	respBody, Err := GetImageFromEJ(ctx, e.Config, imageRatingKey, imageType)
	if Err.Message != "" {
		return imageData, Err
	}

	imageData = respBody
	return imageData, logging.LogErrorInfo{}
}

func (e *EJ) GetCollectionItemImage(ctx context.Context, collection *models.CollectionItem, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Image (%s) for Collection '%s' [%s]",
		e.Config.Type,
		imageType, collection.Title, collection.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	imageData = []byte{}
	Err = logging.LogErrorInfo{}

	respBody, Err := GetImageFromEJ(ctx, e.Config, collection.RatingKey, imageType)
	if Err.Message != "" {
		return imageData, Err
	}

	imageData = respBody
	return imageData, logging.LogErrorInfo{}
}

func GetImageFromEJ(ctx context.Context, msConfig config.Config_MediaServer, ratingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	switch imageType {
	case "poster":
		imageType = "Primary"
	case "backdrop":
		imageType = "Backdrop"
	default:
		return imageData, logging.LogErrorInfo{
			Message: "Invalid image type requested",
			Help:    "Valid types are 'poster' and 'backdrop'",
			Detail:  map[string]any{"requested_type": imageType},
		}
	}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		return imageData, logging.LogErrorInfo{
			Message: "Failed to parse base URL",
			Help:    "Ensure the URL is valid",
			Detail:  map[string]any{"error": err.Error()},
		}
	}
	u.Path = path.Join(u.Path, "Items", ratingKey, "Images", imageType)
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		return imageData, Err
	}
	defer resp.Body.Close()

	// Check if the response body is empty
	if len(respBody) == 0 {
		return imageData, logging.LogErrorInfo{
			Message: "EJ Server returned an empty image response",
			Help:    "The requested image may not exist",
			Detail:  nil,
		}
	}

	imageData = respBody
	return imageData, logging.LogErrorInfo{}
}
