package routes_validation

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type ValidateMediaServerInfo_Request struct {
	MediaServer config.Config_MediaServer `json:"media_server"`
}

type ValidateMediaServerInfo_Response struct {
	Valid       bool                      `json:"valid"`
	Message     string                    `json:"message"`
	MediaServer config.Config_MediaServer `json:"media_server"`
}

// ValidateMediaServerInfo godoc
// @Summary      Validate Media Server Info
// @Description  Validate the provided media server information by attempting to connect to the media server. This endpoint is used during the onboarding process to ensure that the media server settings entered by the user are correct and that a connection can be established. The response will indicate whether the connection was successful and provide details about the media server if it was validated successfully.
// @Tags         Validation
// @Accept       json
// @Produce      json
// @Param        media_server body ValidateMediaServerInfo_Request true "Media Server Information to Validate"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=ValidateMediaServerInfo_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/validate/mediaserver [post]
func ValidateMediaServerInfo(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Validate Media Server Info", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req ValidateMediaServerInfo_Request
	var response ValidateMediaServerInfo_Response

	// Get the Media Server Info from the request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Media Server Info")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	mediaServerInfo := req.MediaServer

	// If the Media Server Token is masked, retrieve the actual token from the config
	if config.IsMaskedField(mediaServerInfo.ApiToken) {
		// A masked token can only be restored for the URL it was issued for. Otherwise the
		// real, live token would be sent to a caller-supplied URL by TestConnection below.
		if mediaServerInfo.URL != config.Current.MediaServer.URL {
			logAction.SetError("Unable to unmask media server credentials", "A new API token must be provided when changing the media server URL", nil)
			httpx.SendResponse(w, ld, response)
			return
		}
		mediaServerInfo.ApiToken = config.Current.MediaServer.ApiToken
	}

	switch mediaServerInfo.Type {
	case "Plex":
	case "Emby", "Jellyfin":
	default:
		logAction.SetError("Unsupported Media Server type: "+mediaServerInfo.Type, "", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	connectionOk, serverName, serverVersion, Err := mediaserver.TestConnection(ctx, &mediaServerInfo)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	} else if !connectionOk {
		logAction.SetError("Failed to connect to media server with provided information", "Please check the media server settings and try again", map[string]any{
			"media_server_info": mediaServerInfo,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	response.MediaServer = mediaServerInfo
	if mediaServerInfo.Type != "Plex" {
		adminUserID, Err := mediaserver.GetAdminUser(ctx, &mediaServerInfo)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		response.MediaServer.UserID = adminUserID
	}
	response.MediaServer.ApiToken = config.MaskToken(response.MediaServer.ApiToken)

	response.Valid = connectionOk
	response.Message = fmt.Sprintf("Successfully connected to %s server '%s' (version %s)", mediaServerInfo.Type, serverName, serverVersion)

	httpx.SendResponse(w, ld, response)
}
