package routes_plex

import (
	"aura/logging"
	"aura/mediaserver/plex"
	"aura/utils/httpx"
	"net/http"
)

type GetPlexPinAndID_Response struct {
	PlexPin string `json:"plex_pin"`
	PlexID  int64  `json:"plex_id"`
}

// GetPlexPinAndID godoc
// @Summary      Get Plex Pin and ID
// @Description  Retrieve a new Plex Pin and the associated Plex ID for authentication. This endpoint is used to initiate the Plex authentication process by providing the necessary credentials for the user to authenticate their Plex account.
// @Tags         Plex
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetPlexPinAndID_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/oauth/plex [get]
func GetPlexPinAndID(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Plex Pin and ID", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetPlexPinAndID_Response

	// Get Pin Code and ID from Plex API
	plexPin, plexID, plexErr := plex.OAuth_GetPinAndID(ctx)
	if plexErr.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.PlexPin = plexPin
	response.PlexID = plexID
	httpx.SendResponse(w, ld, response)
}
