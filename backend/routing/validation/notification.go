package routes_validation

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/notification"
	"aura/utils"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type SendTestNotification_Request struct {
	Provider     config.Config_Notification_Provider `json:"provider"`
	TemplateType string                              `json:"template_type"`
	Template     config.Config_CustomNotification    `json:"template"`
}

type SendTestNotification_Response struct {
	Message string `json:"message"`
}

// SendTestNotification godoc
// @Summary      Send Test Notification
// @Description  Send a test notification using the specified notification provider. This endpoint is used to verify that the notification settings are correct and that notifications can be sent successfully. The request should include the notification provider information, and the response will indicate whether the test notification was sent successfully or if there were any errors.
// @Tags         Validation
// @Accept       json
// @Produce      json
// @Param        provider body SendTestNotification_Request true "Notification Provider Information for sending the test notification"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=SendTestNotification_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/validate/notifications [post]
func SendTestNotification(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Send Test Notification", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req SendTestNotification_Request
	var response SendTestNotification_Response

	// Get the Notification Provider from the request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Notification Provider")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	nProvider := req.Provider

	// If the provider is not enabled, return early
	if !nProvider.Enabled {
		logAction.SetError("Notification provider is not enabled", fmt.Sprintf("%s is not enabled, cannot send test notification", nProvider.Provider), nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Validate the provider settings
	validProvider := config.ValidateNotificationsProvider(ctx, &nProvider)
	if !validProvider {
		httpx.SendResponse(w, ld, response)
		return
	}

	sampleMediaItem := models.MediaItem{
		Title:        "Game of Thrones",
		Year:         2011,
		TMDB_ID:      "1399",
		LibraryTitle: "Series",
		RatingKey:    "1234",
		Type:         "show",
	}

	sampleSet := models.DBPosterSetDetail{
		PosterSet: models.PosterSet{
			BaseSetInfo: models.BaseSetInfo{
				ID:          "7917",
				Title:       "Game of Thrones (2011) Set",
				Type:        "show",
				UserCreated: "willtong93",
			},
		},
	}

	var vars map[string]string
	var title, message, imageURL string
	switch req.TemplateType {
	case config.TemplateTypeAppStartup:
		vars = utils.TemplateVars_AppStartup(config.AppName, config.AppVersion, config.AppPort)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""

	case config.TemplateTypeTestNotification:
		vars = utils.TemplateVars_TestNotification()
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	case config.TemplateTypeAutodownload:
		vars = utils.MergeTemplateVars(
			utils.BaseTemplateVars(),
			map[string]string{
				"MediaItemTitle":        sampleMediaItem.Title,
				"MediaItemYear":         fmt.Sprintf("%d", sampleMediaItem.Year),
				"MediaItemTMDBID":       sampleMediaItem.TMDB_ID,
				"MediaItemLibraryTitle": sampleMediaItem.LibraryTitle,
				"MediaItemRatingKey":    sampleMediaItem.RatingKey,
				"MediaItemType":         sampleMediaItem.Type,
				"SetID":                 sampleSet.ID,
				"SetTitle":              sampleSet.Title,
				"SetType":               sampleSet.Type,
				"SetCreator":            sampleSet.UserCreated,
				"ImageName":             "S01E01 Titlecard",
				"ImageType":             "titlecard",
				"ReasonTitle":           "Episode Changed",
				"Reason":                "Season 01 Episode 01 changed since last download\nChange detected in episode info:\nPath changed:\n-old: /path/to/old/file.mkv\n-new: /path/to/new/file.mkv",
			},
		)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	case config.TemplateTypeDownloadQueue:
		vars = utils.MergeTemplateVars(
			utils.BaseTemplateVars(),
			map[string]string{
				"MediaItemTitle":        sampleMediaItem.Title,
				"MediaItemYear":         fmt.Sprintf("%d", sampleMediaItem.Year),
				"MediaItemTMDBID":       sampleMediaItem.TMDB_ID,
				"MediaItemLibraryTitle": sampleMediaItem.LibraryTitle,
				"MediaItemRatingKey":    sampleMediaItem.RatingKey,
				"MediaItemType":         sampleMediaItem.Type,
				"SetID":                 sampleSet.ID,
				"SetTitle":              sampleSet.Title,
				"SetType":               sampleSet.Type,
				"SetCreator":            sampleSet.UserCreated,
				"ReasonTitle":           "Success",
				"Reason":                "Download completed successfully with no issues detected.",
			},
		)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	case config.TemplateTypeNewSetsAvailableForIgnoredItems:
		vars = utils.MergeTemplateVars(
			utils.BaseTemplateVars(),
			map[string]string{
				"MediaItemTitle":        sampleMediaItem.Title,
				"MediaItemYear":         fmt.Sprintf("%d", sampleMediaItem.Year),
				"MediaItemTMDBID":       sampleMediaItem.TMDB_ID,
				"MediaItemLibraryTitle": sampleMediaItem.LibraryTitle,
				"MediaItemRatingKey":    sampleMediaItem.RatingKey,
				"MediaItemType":         sampleMediaItem.Type,
				"SetCount":              "3",
			},
		)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	case config.TemplateTypeCheckForMediaItemChangesJob:
		vars = utils.MergeTemplateVars(
			utils.BaseTemplateVars(),
			map[string]string{
				"MediaItemTitle":        sampleMediaItem.Title,
				"MediaItemYear":         fmt.Sprintf("%d", sampleMediaItem.Year),
				"MediaItemTMDBID":       sampleMediaItem.TMDB_ID,
				"MediaItemLibraryTitle": sampleMediaItem.LibraryTitle,
				"MediaItemRatingKey":    sampleMediaItem.RatingKey,
				"MediaItemType":         sampleMediaItem.Type,
				"Reason":                "This item was not in any Saved Sets and does not have a status of Ignored.",
				"Action":                "This item will be removed from the database since it is not in the media server cache and does not have any Saved Sets or Ignored status",
				"MoreInfo":              "This may indicate that the media item was removed from the media server or there is an issue with the media server cache. Please verify if this media item still exists in the media server. If it does exist and you want to keep it in the database, please add it to a Saved Set or set it to be ignored temporarily.",
			},
		)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	case config.TemplateTypeSonarrNotification:
		vars = utils.MergeTemplateVars(
			utils.BaseTemplateVars(),
			map[string]string{
				"MediaItemTitle":        sampleMediaItem.Title,
				"MediaItemYear":         fmt.Sprintf("%d", sampleMediaItem.Year),
				"MediaItemTMDBID":       sampleMediaItem.TMDB_ID,
				"MediaItemLibraryTitle": sampleMediaItem.LibraryTitle,
				"MediaItemRatingKey":    sampleMediaItem.RatingKey,
				"MediaItemType":         sampleMediaItem.Type,
				"SetID":                 sampleSet.ID,
				"SetTitle":              sampleSet.Title,
				"SetType":               sampleSet.Type,
				"SetCreator":            sampleSet.UserCreated,
				"ImageName":             "S01E01 Titlecard",
				"ImageType":             "titlecard",
				"ReasonTitle":           "New Download",
				"Reason":                "A new episode was downloaded via Sonarr for this media item.",
				"Result":                "Success",
			},
		)
		title = utils.RenderTemplate(req.Template.Title, vars)
		message = utils.RenderTemplate(req.Template.Message, vars)
		imageURL = ""
	default:
		logAction.SetError("Unsupported template type", fmt.Sprintf("The template type '%s' is not supported", req.TemplateType), nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	if !req.Template.Enabled {
		response.Message = fmt.Sprintf("Test notification template '%s' is not enabled, skipping sending test notification", req.TemplateType)
		httpx.SendResponse(w, ld, response)
		return
	}

	switch nProvider.Provider {
	case "Discord":
		webhook := nProvider.Discord.Webhook
		if config.IsMaskedWebhook(webhook) {
			unmasked := getUnmaskedDiscordWebhook(webhook)
			if unmasked == "" {
				logAction.SetError("Unable to unmask Discord webhook", "Please provide the full Discord webhook URL", nil)
				httpx.SendResponse(w, ld, response)
				return
			}
			nProvider.Discord.Webhook = unmasked
		}
		Err := notification.SendDiscordMessage(ctx, nProvider.Discord, message, imageURL, title)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	case "Pushover":
		userKey := nProvider.Pushover.UserKey
		apiToken := nProvider.Pushover.ApiToken
		if config.IsMaskedField(userKey) {
			userKey = getUnmaskedPushoverField("UserKey", userKey)
		}
		if config.IsMaskedField(apiToken) {
			apiToken = getUnmaskedPushoverField("Token", apiToken)
		}
		if userKey == "" || apiToken == "" {
			logAction.SetError("Unable to unmask Pushover credentials", "Please provide the full Pushover UserKey and ApiToken", nil)
			httpx.SendResponse(w, ld, response)
			return
		}
		nProvider.Pushover.UserKey = userKey
		nProvider.Pushover.ApiToken = apiToken
		Err := notification.SendPushoverMessage(ctx, nProvider.Pushover, message, imageURL, title)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	case "Gotify":
		url := nProvider.Gotify.URL
		apiToken := nProvider.Gotify.ApiToken
		if config.IsMaskedField(url) {
			url = getUnmaskedGotifyField("URL", url)
		}
		if config.IsMaskedField(apiToken) {
			// A masked token can only be restored for the URL it was issued for. Otherwise the
			// real, live token would be sent to a caller-supplied URL by SendGotifyMessage below.
			if url != storedGotifyURL() {
				logAction.SetError("Unable to unmask Gotify credentials", "A new API token must be provided when changing the Gotify URL", nil)
				httpx.SendResponse(w, ld, response)
				return
			}
			apiToken = getUnmaskedGotifyField("Token", apiToken)
		}
		if url == "" || apiToken == "" {
			logAction.SetError("Unable to unmask Gotify credentials", "Please provide the full Gotify URL and ApiToken", nil)
			httpx.SendResponse(w, ld, response)
			return
		}
		nProvider.Gotify.URL = url
		nProvider.Gotify.ApiToken = apiToken
		Err := notification.SendGotifyMessage(ctx, nProvider.Gotify, message, imageURL, title)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	case "Webhook":
		Err := notification.SendWebhookMessage(ctx, nProvider.Webhook, message, imageURL, title)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	default:
		logAction.SetError("Unsupported notification provider", fmt.Sprintf("The notification provider '%s' is not supported for test messages", nProvider.Provider), nil)
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Message = fmt.Sprintf("Test notification sent successfully via %s", nProvider.Provider)
	httpx.SendResponse(w, ld, response)
}

func getUnmaskedPushoverField(field, currentValue string) string {
	for _, existingProvider := range config.Current.Notifications.Providers {
		if existingProvider.Provider == "Pushover" && existingProvider.Pushover != nil {
			switch field {
			case "UserKey":
				if existingProvider.Pushover.UserKey != "" {
					// Make sure that the last few characters match the masked value
					if len(currentValue) > 3 && len(existingProvider.Pushover.UserKey) >= 3 {
						if currentValue[len(currentValue)-3:] == existingProvider.Pushover.UserKey[len(existingProvider.Pushover.UserKey)-3:] {
							return existingProvider.Pushover.UserKey
						}
					}
				}
			case "Token":
				if existingProvider.Pushover.ApiToken != "" {
					// Make sure that the last few characters match the masked value
					if len(currentValue) > 3 && len(existingProvider.Pushover.ApiToken) >= 3 {
						if currentValue[len(currentValue)-3:] == existingProvider.Pushover.ApiToken[len(existingProvider.Pushover.ApiToken)-3:] {
							return existingProvider.Pushover.ApiToken
						}
					}
				}
			}
		}
	}
	return ""
}

func getUnmaskedDiscordWebhook(currentValue string) string {
	for _, existingProvider := range config.Current.Notifications.Providers {
		if existingProvider.Provider == "Discord" && existingProvider.Discord != nil {
			if existingProvider.Discord.Webhook != "" {
				// Make sure that the last few characters match the masked value
				if len(currentValue) > 3 && len(existingProvider.Discord.Webhook) >= 3 {
					if currentValue[len(currentValue)-3:] == existingProvider.Discord.Webhook[len(existingProvider.Discord.Webhook)-3:] {
						return existingProvider.Discord.Webhook
					}
				}
			}
		}
	}
	return ""
}

func storedGotifyURL() string {
	for _, existingProvider := range config.Current.Notifications.Providers {
		if existingProvider.Provider == "Gotify" && existingProvider.Gotify != nil {
			return existingProvider.Gotify.URL
		}
	}
	return ""
}

func getUnmaskedGotifyField(field, currentValue string) string {
	for _, existingProvider := range config.Current.Notifications.Providers {
		if existingProvider.Provider == "Gotify" && existingProvider.Gotify != nil {
			switch field {
			case "URL":
				if existingProvider.Gotify.URL != "" {
					// Make sure that the last few characters match the masked value
					if len(currentValue) > 3 && len(existingProvider.Gotify.URL) >= 3 {
						if currentValue[len(currentValue)-3:] == existingProvider.Gotify.URL[len(existingProvider.Gotify.URL)-3:] {
							return existingProvider.Gotify.URL
						}
					}
				}
			case "Token":
				if existingProvider.Gotify.ApiToken != "" {
					// Make sure that the last few characters match the masked value
					if len(currentValue) > 3 && len(existingProvider.Gotify.ApiToken) >= 3 {
						if currentValue[len(currentValue)-3:] == existingProvider.Gotify.ApiToken[len(existingProvider.Gotify.ApiToken)-3:] {
							return existingProvider.Gotify.ApiToken
						}
					}
				}
			}
		}
	}
	return ""
}
