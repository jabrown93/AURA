package silo

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func applyImageToMediaItem(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itemRatingKey, imageURL, imageType string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Applying '%s' Image to %s",
		cases.Title(language.English).String(imageType), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Season posters use the poster type on the season, and titlecards use the still type on the episode
	siloImageType := map[string]string{
		"poster":                "poster",
		"backdrop":              "backdrop",
		"season_poster":         "poster",
		"special_season_poster": "poster",
		"titlecard":             "still",
	}[imageType]
	if siloImageType == "" {
		logAction.SetError("Unsupported image type for Silo", "Supported types are poster, backdrop, season_poster, special_season_poster and titlecard",
			map[string]any{"image_type": imageType})
		return *logAction.Error
	}

	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "admin", "items", itemRatingKey, "images", "apply")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	body, _ := json.Marshal(SiloImageApplyRequest{
		Type:        siloImageType,
		OriginalURL: imageURL,
	})

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "POST", body)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// Decode the Response
	var siloResp SiloImageApplyResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Image Apply Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	logAction.AppendResult("stored_path", siloResp.StoredPath)

	return logging.LogErrorInfo{}
}
