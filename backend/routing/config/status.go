package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/utils/httpx"
	"context"
	"net/http"
	"time"
)

// AppConfigStatus represents the current status of the onboarding process and app config
type AppConfigStatus struct {
	ConfigLoaded      bool          `json:"config_loaded"`                 // Whether a config file is loaded
	ConfigValid       bool          `json:"config_valid"`                  // Whether the loaded config is valid
	NeedsSetup        bool          `json:"needs_setup"`                   // Whether the app needs initial setup
	CurrentSetup      config.Config `json:"current_setup"`                 // The current (sanitized) configuration
	MediaServerName   string        `json:"media_server_name,omitempty"`   // Friendly name of the media server
	MediuxSiteLink    string        `json:"mediux_site_link,omitempty"`    // Current Mediux site link
	AppFullyLoaded    bool          `json:"app_fully_loaded"`              // Whether the app is fully loaded and ready to use
	AppVersion        string        `json:"app_version"`                   // Current version of the app
	AppLoadingStep    string        `json:"app_loading_step"`              // Current loading step of the app
	APIKeyConfigured  bool          `json:"api_key_configured"`            // Whether a global API key has been generated (the key itself is never exposed)
	AuthDisabledByEnv bool          `json:"auth_disabled_by_env"`          // Whether AURA_DISABLE_AUTH has turned off authentication
	SetupCodeRequired bool          `json:"setup_code_required,omitempty"` // Whether the setup code must be entered first
	ConfigHasErrors   bool          `json:"config_has_errors"`             // Whether the required settings are entered, but have errors
	// Fork: an outage is a degraded state shown in the navbar, not a reason to open setup.
	MediaServerReachable bool `json:"media_server_reachable"`
	MediuxReachable      bool `json:"mediux_reachable"`
	MediuxValid          bool `json:"mediux_valid"`
}

type configStatusResponse struct {
	Status AppConfigStatus `json:"status"`
}

// Status godoc
// @Summary      Get Config Status
// @Description  Get the current status of the app configuration and onboarding process
// @Tags         Config
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=routes_config.configStatusResponse}
// @Router       /api/config [get]
func GetAppConfigStatus(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Config Status", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response configStatusResponse

	response.Status = currentConfigStatus(ctx)
	httpx.SendResponse(w, ld, response)
}

// currentConfigStatus returns the current sanitized config status
func currentConfigStatus(ctx context.Context) AppConfigStatus {
	currentConfig := config.Current // Make a local copy of the current config for sanitization

	return AppConfigStatus{
		ConfigLoaded:         config.Loaded,
		ConfigValid:          (config.Valid && config.MediuxValid && config.MediaServerValid),
		NeedsSetup:           !(config.Loaded && config.Valid && config.MediuxValid && config.MediaServerValid),
		CurrentSetup:         *currentConfig.SanitizeConfig(ctx),
		MediaServerReachable: config.MediaServerReachable,
		MediuxReachable:      config.MediuxReachable,
		MediuxValid:          config.MediuxValid,
		MediaServerName:      config.MediaServerName,
		MediuxSiteLink:       mediux.MediuxSiteLink,
		AppFullyLoaded:       config.AppFullyLoaded,
		AppVersion:           config.AppVersion,
		AppLoadingStep:       config.AppLoadingStep,
		APIKeyConfigured:     config.Current.Auth.APIKeyHash != "",
		AuthDisabledByEnv:    config.AuthDisabledByEnv(),
		ConfigHasErrors:      config.Loaded && !(config.Valid && config.MediuxValid && config.MediaServerValid) && config.Current.IsSetUp(),
	}
}

// GetSetupConfigStatus godoc
// @Summary      Get Config Status (Setup Mode)
// @Description  Get the current status of the app configuration while aura is in setup mode. Without the setup code (X-Setup-Code header), only the app status is returned, not the config.
// @Tags         Config
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=routes_config.configStatusResponse}
// @Router       /api/config [get]
func GetSetupConfigStatus(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Config Status (Setup Mode)", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)

	httpx.SendResponse(w, ld, configStatusResponse{Status: setupConfigStatus(ctx)})
}

// setupConfigStatus returns the config status in setup mode.
// Without a login, the config is only returned once the setup code is entered.
func setupConfigStatus(ctx context.Context) AppConfigStatus {
	if !config.AuthEnabled() && !httpx.SetupCodeVerified(ctx) {
		defaults := config.DefaultConfig()
		status := currentConfigStatus(ctx)
		status.CurrentSetup = *defaults.SanitizeConfig(ctx)
		status.MediaServerName = ""
		status.APIKeyConfigured = false
		status.SetupCodeRequired = config.Loaded
		return status
	}
	return currentConfigStatus(ctx)
}

type verifySetupCodeRequest struct {
	Code string `json:"code"`
}

type verifySetupCodeResponse struct {
	Valid bool `json:"valid"`
}

// VerifySetupCode godoc
// @Summary      Verify Setup Code
// @Description  Check the setup code printed in the aura logs. The setup page then sends it with each request (X-Setup-Code header) while aura is in setup mode.
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        code  body      verifySetupCodeRequest  true  "Setup code"
// @Success      200   {object}  httpx.JSONResponse{data=routes_config.verifySetupCodeResponse}
// @Failure      500   {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/setup/verify [post]
func VerifySetupCode(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Verify Setup Code", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req verifySetupCodeRequest
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Verify Setup Code")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, verifySetupCodeResponse{})
		return
	}

	if !config.CheckSetupCode(req.Code) {
		// Slow down guessing
		time.Sleep(500 * time.Millisecond)
		logAction.SetError("Incorrect setup code", "Enter the setup code from the aura logs (e.g. K7QM-3RXA)", nil)
		httpx.SendResponse(w, ld, verifySetupCodeResponse{})
		return
	}
	httpx.SendResponse(w, ld, verifySetupCodeResponse{Valid: true})
}
