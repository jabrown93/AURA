package plex

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

func (p *Plex) GetMediaItemImage(ctx context.Context, item *models.MediaItem, imageRatingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Fetching Image (%s) for %s",
		imageType, utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	imageData = []byte{}
	Err = logging.LogErrorInfo{}

	respBody, Err := GetImageFromPlex(ctx, imageRatingKey, imageType)
	if Err.Message != "" {
		return imageData, Err
	}

	imageData = respBody
	return imageData, Err
}

func (p *Plex) GetCollectionItemImage(ctx context.Context, collection *models.CollectionItem, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Fetching Image (%s) for Collection '%s' [%s]",
		imageType, collection.Title, collection.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	imageData = []byte{}
	Err = logging.LogErrorInfo{}

	respBody, Err := GetImageFromPlex(ctx, collection.RatingKey, imageType)
	if Err.Message != "" {
		return imageData, Err
	}

	imageData = respBody
	return imageData, Err
}

func GetImageFromPlex(ctx context.Context, ratingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	width := "600"
	height := "900"
	switch imageType {
	case "backdrop":
		imageType = "art"
		width = "1280"
		height = "720"
	case "thumb":
		width = "400"
		height = "225"
	}

	// Construct the URL for the Plex Image request
	photoPath := path.Join("/library/metadata", ratingKey, imageType)
	encodedPhotoPath := url.QueryEscape(photoPath)
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		return imageData, logging.LogErrorInfo{
			Message: "Failed to parse base URL",
			Help:    "Ensure the URL is valid",
			Detail:  map[string]any{"error": err.Error()},
		}
	}
	u.Path = "/photo/:/transcode"
	query := u.Query()
	query.Set("width", width)
	query.Set("height", height)
	u.RawQuery = query.Encode()
	URL := u.String()
	URL = fmt.Sprintf("%s&url=%s", URL, encodedPhotoPath)

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		return imageData, Err
	}
	defer resp.Body.Close()

	// Check if response has data
	if len(respBody) == 0 {
		return imageData, logging.LogErrorInfo{
			Message: "Plex Server returned an empty image response",
			Help:    "The requested image may not exist",
			Detail:  nil,
		}
	}

	imageData = respBody
	return imageData, Err
}
