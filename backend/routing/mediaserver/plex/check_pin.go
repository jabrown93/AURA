package routes_plex

import (
	"aura/logging"
	"aura/mediaserver/plex"
	"aura/utils/httpx"
	"net/http"
)

type CheckAuthStatusWithPlex_Response struct {
	Authenticated        bool                       `json:"authenticated"`
	AuthToken            string                     `json:"auth_token"`
	ConnectionsAvailable []plex.PlexServersResponse `json:"connections_available"`
}

// CheckAuthStatusWithPlex godoc
// @Summary      Check Plex Pin for Authentication
// @Description  Check if the provided Plex ID has authenticated with Plex and retrieve available server connections if authenticated. This endpoint is used during the Plex authentication process to verify the user's Plex account and gather necessary information for integration.
// @Tags         Plex
// @Accept       json
// @Produce      json
// @Param        plex_id  query     string  true  "Plex ID to check for authentication"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200      {object}  httpx.JSONResponse{data=CheckAuthStatusWithPlex_Response}
// @Failure      500      {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/oauth/plex [post]
func CheckAuthStatusWithPlex(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Check Plex ID For Auth", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response CheckAuthStatusWithPlex_Response
	// Get the Plex ID from the request
	plexID := r.URL.Query().Get("plex_id")
	if plexID == "" {
		logAction.SetError("Plex ID is required", "No Plex ID provided in request", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Check the Plex ID to see if the user has authenticated with Plex
	isValid, authToken, plexErr := plex.OAuth_CheckIDForAuth(ctx, plexID)
	if plexErr.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// If the Plex ID is valid, get a list of available connections
	var listOfPlexServerConnections []plex.PlexServersResponse
	if isValid && authToken != "" {
		listOfPlexServerConnections, plexErr = plex.OAuth_GetServerConnections(ctx, authToken)
		if plexErr.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	}

	response.Authenticated = isValid
	response.AuthToken = authToken
	response.ConnectionsAvailable = listOfPlexServerConnections
	httpx.SendResponse(w, ld, response)
}
