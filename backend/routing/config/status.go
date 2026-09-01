package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/utils/httpx"
	"net/http"
)

// AppConfigStatus represents the current status of the onboarding process and app config
type AppConfigStatus struct {
	ConfigLoaded         bool          `json:"config_loaded"`               // Whether a config file is loaded
	ConfigValid          bool          `json:"config_valid"`                // Whether the loaded config is valid
	NeedsSetup           bool          `json:"needs_setup"`                 // Whether the app needs initial setup
	MediaServerReachable bool          `json:"media_server_reachable"`      // Whether the media server answered the last check
	MediuxReachable      bool          `json:"mediux_reachable"`            // Whether MediUX answered the last check
	CurrentSetup         config.Config `json:"current_setup"`               // The current (sanitized) configuration
	MediaServerName      string        `json:"media_server_name,omitempty"` // Friendly name of the media server
	MediuxSiteLink       string        `json:"mediux_site_link,omitempty"`  // Current Mediux site link
	AppFullyLoaded       bool          `json:"app_fully_loaded"`            // Whether the app is fully loaded and ready to use
	AppVersion           string        `json:"app_version"`                 // Current version of the app
	AppLoadingStep       string        `json:"app_loading_step"`            // Current loading step of the app
}

type configStatusResponse struct {
	Status AppConfigStatus `json:"status"`
}

// Status godoc
// @Summary      Get Config Status
// @Description  Get the current status of the app configuration and onboarding process
// @Tags         Config
// @Produce      json
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=routes_config.configStatusResponse}
// @Router       /api/config [get]
func GetAppConfigStatus(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Config Status", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response configStatusResponse

	currentConfig := config.Current // Make a local copy of the current config for sanitization

	response.Status = AppConfigStatus{
		ConfigLoaded:         config.Loaded,
		ConfigValid:          config.Valid,
		NeedsSetup:           config.NeedsSetup(),
		MediaServerReachable: config.MediaServerReachable,
		MediuxReachable:      config.MediuxReachable,
		CurrentSetup:         *currentConfig.SanitizeConfig(ctx),
		MediaServerName:      config.MediaServerName,
		MediuxSiteLink:       mediux.MediuxSiteLink,
		AppFullyLoaded:       config.AppFullyLoaded,
		AppVersion:           config.AppVersion,
		AppLoadingStep:       config.AppLoadingStep,
	}
	httpx.SendResponse(w, ld, response)
}
