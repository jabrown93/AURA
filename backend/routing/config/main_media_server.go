package routes_config

import (
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"net/http"
)

type mainMediaServerRequest struct {
	ID string `json:"id"` // ID of the media server to make the main one
}

type mainMediaServerResponse struct {
	Plan   mediaserver.SwitchMainPlan `json:"plan"`
	Status *AppConfigStatus           `json:"status,omitempty"` // Set once the main media server has changed
}

// PreviewMainMediaServerSwitch godoc
// @Summary      Preview Main Media Server Switch
// @Description  Show what happens to the saved items if another media server becomes the main one. Nothing is changed.
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        server  body      mainMediaServerRequest  true  "Media server"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200     {object}  httpx.JSONResponse{data=routes_config.mainMediaServerResponse}
// @Failure      500     {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/main-media-server/preview [post]
func PreviewMainMediaServerSwitch(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Preview Main Media Server Switch", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req mainMediaServerRequest
	var response mainMediaServerResponse
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Main Media Server"); Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	plan, Err := mediaserver.PlanMainMediaServerSwitch(ctx, req.ID)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
	}
	response.Plan = plan
	httpx.SendResponse(w, ld, response)
}

// SwitchMainMediaServer godoc
// @Summary      Switch Main Media Server
// @Description  Make another media server the main one. The saved items move to its items, and the library maps change to its libraries.
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        server  body      mainMediaServerRequest  true  "Media server"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200     {object}  httpx.JSONResponse{data=routes_config.mainMediaServerResponse}
// @Failure      500     {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/main-media-server [post]
func SwitchMainMediaServer(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Switch Main Media Server", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req mainMediaServerRequest
	var response mainMediaServerResponse
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Main Media Server"); Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	plan, Err := mediaserver.SwitchMainMediaServer(ctx, req.ID)
	response.Plan = plan
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		httpx.SendResponse(w, ld, response)
		return
	}

	// The Plex event listener follows the main media server
	restartPlexEventListener()

	status := currentConfigStatus(ctx)
	response.Status = &status
	httpx.SendResponse(w, ld, response)
}
