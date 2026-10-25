package routes_validation

import (
	"aura/config"
	"aura/logging"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils/httpx"
	"net/http"
)

type ValidateSonarrRadarrInfo_Request struct {
	SonarrRadarrInfo config.Config_SonarrRadarrApp `json:"sonarr_radarr_info"`
}

type ValidateSonarrRadarrInfo_Response struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateSonarrRadarrInfo godoc
// @Summary      Validate Sonarr/Radarr Information
// @Description  Validate the provided Sonarr/Radarr information by attempting to connect to the application. This endpoint is used during the onboarding process to ensure that the Sonarr/Radarr settings entered by the user are correct and that a connection can be established. The response will indicate whether the connection was successful and provide details about the application if it was validated successfully.
// @Tags         Validation
// @Accept       json
// @Produce      json
// @Param        sonarr_radarr_info body ValidateSonarrRadarrInfo_Request true "Sonarr/Radarr Information to Validate"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=ValidateSonarrRadarrInfo_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/validate/sonarr [post]
// @Router       /api/validate/radarr [post]
func ValidateSonarrRadarrInfo(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Validate Sonarr/Radarr Info", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req ValidateSonarrRadarrInfo_Request
	var response ValidateSonarrRadarrInfo_Response

	// Get the Sonarr/Radarr information from the request
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Sonarr/Radarr Info")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	srInfo := req.SonarrRadarrInfo

	fullToken := srInfo.ApiToken
	if config.IsMaskedField(srInfo.ApiToken) {
		// If the token is masked, we need to grab it from the existing config
		for _, existingApp := range config.Current.SonarrRadarr.Applications {
			if existingApp.Type == srInfo.Type && existingApp.URL == srInfo.URL {
				fullToken = existingApp.ApiToken
				break
			}
		}
	}
	srInfo.ApiToken = fullToken

	_, Err = sonarr_radarr.TestConnection(ctx, srInfo)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	srInfo.ApiToken = config.MaskToken(srInfo.ApiToken)
	response.Valid = true
	response.Message = "Sonarr/Radarr information is valid"
	httpx.SendResponse(w, ld, response)
}
