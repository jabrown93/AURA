package silo

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

func (s *Silo) GetMediaItemImage(ctx context.Context, item *models.MediaItem, imageRatingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Image (%s) for %s",
		imageType,
		utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	imageData = []byte{}
	Err = logging.LogErrorInfo{}

	respBody, Err := GetImageFromSilo(ctx, s.Config, imageRatingKey, imageType)
	if Err.Message != "" {
		return imageData, Err
	}

	imageData = respBody
	return imageData, logging.LogErrorInfo{}
}

func GetImageFromSilo(ctx context.Context, msConfig config.Config_MediaServer, ratingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	var imageSize string
	switch imageType {
	case "poster", "backdrop":
		imageSize = "large"
	case "thumb":
		imageSize = "medium"
	default:
		return imageData, logging.LogErrorInfo{
			Message: "Invalid image type requested",
			Help:    "Valid types are 'poster', 'backdrop' and 'thumb'",
			Detail:  map[string]any{"requested_type": imageType},
		}
	}

	// Get the Media Item details from Silo
	siloResp, Err := getCatalogItemDetail(ctx, msConfig, ratingKey, imageSize)
	if Err.Message != "" {
		return imageData, Err
	}

	// Episodes return their still image as the poster
	artworkPath := siloResp.PosterURL
	if imageType == "backdrop" {
		artworkPath = siloResp.BackdropURL
	}
	if artworkPath == "" {
		return imageData, logging.LogErrorInfo{
			Message: "Silo Server does not have this image",
			Help:    "The requested image may not exist",
			Detail:  map[string]any{"rating_key": ratingKey, "image_type": imageType},
		}
	}
	return getArtwork(ctx, msConfig, artworkPath)
}

// getArtwork downloads artwork from the signed path Silo gives
func getArtwork(ctx context.Context, msConfig config.Config_MediaServer, artworkPath string) (imageData []byte, Err logging.LogErrorInfo) {
	// Construct the URL for the Silo artwork request
	artworkURL, err := url.Parse(artworkPath)
	if err != nil {
		return imageData, logging.LogErrorInfo{
			Message: "Failed to parse Silo artwork URL",
			Help:    "Check the response from the server for more details",
			Detail:  map[string]any{"artwork_url": artworkPath, "error": err.Error()},
		}
	}
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		message, help, detail := logging.Error_BaseUrlParsing(err)
		return imageData, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
	}
	u.Path = path.Join(u.Path, artworkURL.Path)
	u.RawQuery = artworkURL.RawQuery

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return imageData, Err
	}

	// Check if the response body is empty
	if len(respBody) == 0 {
		return imageData, logging.LogErrorInfo{
			Message: "Silo Server returned an empty image response",
			Help:    "The requested image may not exist",
			Detail:  nil,
		}
	}

	imageData = respBody
	return imageData, logging.LogErrorInfo{}
}
