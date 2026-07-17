package routes_ms

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
	"strings"
)

type getLibrarySectionOptionsRequest struct {
	MediaServer config.Config_MediaServer `json:"media_server"`
}

type getLibrarySectionOptionsResponse struct {
	LibrarySections []models.LibrarySection `json:"library_sections"`
}

// GetLibrarySectionOptions godoc
// @Summary      Get Library Section Options
// @Description  Retrieve a list of library sections from a specified media server configuration. This endpoint accepts a media server configuration in the request body and returns the available library sections for that media server, allowing clients to display options for users to select which library section they want to interact with.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        request body getLibrarySectionOptionsRequest true "Media Server Configuration"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=getLibrarySectionOptionsResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/libraries/options [post]
func GetLibrarySectionOptions(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Library Section Options", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req getLibrarySectionOptionsRequest
	var response getLibrarySectionOptionsResponse

	// Get the reqeuest body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Get Library Section Options - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	msConfig := req.MediaServer

	if strings.HasPrefix(msConfig.ApiToken, "***") {
		// A masked token can only be restored for the URL it was issued for. Otherwise the
		// real, live token would be sent to a caller-supplied URL by GetLibrarySections below.
		if msConfig.URL != config.Current.MediaServer.URL {
			logAction.SetError("Unable to unmask media server credentials", "A new API token must be provided when changing the media server URL", nil)
			httpx.SendResponse(w, ld, response)
			return
		}
		msConfig.ApiToken = config.Current.MediaServer.ApiToken
	}

	// Get all available library sections from the Media Server
	response.LibrarySections, Err = mediaserver.GetLibrarySections(ctx, &msConfig)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if len(response.LibrarySections) == 0 {
		logAction.SetError("No library sections found", "No library sections could be retrieved from the Media Server", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	httpx.SendResponse(w, ld, response)
}
