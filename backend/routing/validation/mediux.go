package routes_validation

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/utils/httpx"
	"net/http"
)

type ValidateMediuxInfo_Request struct {
	MediuxInfo config.Config_Mediux `json:"mediux_info"`
}

type ValidateMediuxInfo_Response struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateMediuxInfo godoc
// @Summary      Validate Mediux Token
// @Description  Validate the provided Mediux API token by attempting to connect to the Mediux site. This endpoint is used during the onboarding process to ensure that the Mediux settings entered by the user are correct and that a connection can be established. The response will indicate whether the connection was successful.
// @Tags         Validation
// @Accept       json
// @Produce      json
// @Param        mediux_info body ValidateMediuxInfo_Request true "Mediux Information to Validate"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=ValidateMediuxInfo_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/validate/mediux [post]
func ValidateMediuxInfo(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Validate Mediux Token", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req ValidateMediuxInfo_Request
	var response ValidateMediuxInfo_Response

	// Get the MediUX Info from the request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Mediux Info")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	mediuxInfo := req.MediuxInfo

	// If the MediUX Token is masked, retrieve the actual token from the config
	if config.IsMaskedField(mediuxInfo.ApiToken) {
		mediuxInfo.ApiToken = config.Current.Mediux.ApiToken
	}

	isValid, Err := mediux.ValidateToken(ctx, mediuxInfo.ApiToken)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Valid = isValid
	response.Message = "Successfully validated Mediux token"
	httpx.SendResponse(w, ld, response)
}
