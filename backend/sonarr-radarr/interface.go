package sonarr_radarr

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
)

type SonarrRadarrTag struct {
	Label string `json:"label"`
	ID    int    `json:"id"`
}

type SonarrRadarrInterface interface {
	// Test Connection
	TestConnection(ctx context.Context, app config.Config_SonarrRadarrApp) (valid bool, Err logging.LogErrorInfo)

	// Get Sonarr/Radarr Item Info from the item TMDB ID
	GetItemInfoFromTMDBID(ctx context.Context, app config.Config_SonarrRadarrApp, tmdbID int) (resp any, Err logging.LogErrorInfo)

	// Get configured tags from Sonarr/Radarr
	GetAllTags(ctx context.Context, app config.Config_SonarrRadarrApp) (tags []SonarrRadarrTag, Err logging.LogErrorInfo)

	// Add new tags to Sonarr/Radarr
	AddNewTags(ctx context.Context, app config.Config_SonarrRadarrApp, newTags []string) (tags []SonarrRadarrTag, Err logging.LogErrorInfo)

	// Update tags on Sonarr/Radarr item
	HandleTags(ctx context.Context, app config.Config_SonarrRadarrApp, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo)
}

type SonarrApp struct{}
type RadarrApp struct{}

func MakeSureAllAppInfoPresent(ctx context.Context, app *config.Config_SonarrRadarrApp) (Err logging.LogErrorInfo) {
	if app.Type == "" || app.URL == "" || app.ApiToken == "" || app.Library == "" {
		_, logAction := logging.AddSubActionToContext(ctx, "Making sure Sonarr/Radar Info is set", logging.LevelTrace)
		logAction.SetError("Sonarr/Radarr App Info Incomplete",
			"Ensure the Sonarr/Radarr Type, Library, URL, and API Token are set in the config file",
			map[string]any{
				"type":      app.Type,
				"library":   app.Library,
				"url":       app.URL,
				"api_token": config.MaskToken(app.ApiToken),
			})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

func NewSonarrRadarrInterface(ctx context.Context, app config.Config_SonarrRadarrApp) (SonarrRadarrInterface, logging.LogErrorInfo) {
	var interfaceSR SonarrRadarrInterface
	switch app.Type {
	case "Sonarr":
		interfaceSR = &SonarrApp{}
	case "Radarr":
		interfaceSR = &RadarrApp{}
	default:
		_, logAction := logging.AddSubActionToContext(ctx, "Creating Sonarr/Radarr Interface", logging.LevelTrace)
		logAction.SetError(fmt.Sprintf("Unsupported Sonarr/Radarr Type: '%s'", app.Type),
			"Ensure the Sonarr/Radarr Type is set to either 'Sonarr' or 'Radarr' in the config file",
			nil)
		return nil, *logAction.Error
	}

	// Make sure all required info is set
	Err := MakeSureAllAppInfoPresent(ctx, &app)
	if Err.Message != "" {
		return nil, Err
	}

	return interfaceSR, logging.LogErrorInfo{}
}
