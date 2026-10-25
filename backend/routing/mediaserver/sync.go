package routes_ms

import (
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"net/http"
)

type startMediaServerSyncRequest struct {
	ServerID string `json:"server_id"` // The media server to apply the saved sets to
}

type mediaServerSyncResponse struct {
	Sync *mediaserver.MediaServerSync `json:"sync"` // The latest sync, null if there hasn't been one
}

// StartMediaServerSync godoc
// @Summary      Apply Saved Sets to a Media Server
// @Description  Apply every saved set to a media server that isn't the main one. It runs in the background, use GET /api/mediaserver/sync for its progress.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        req  body  startMediaServerSyncRequest  true  "The media server"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=mediaServerSyncResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/sync [post]
func StartMediaServerSync(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Start Applying Saved Sets to Media Server", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req startMediaServerSyncRequest
	var response mediaServerSyncResponse
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Start Media Server Sync - Decode Request Body"); Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	state, err := mediaserver.StartMediaServerSync(req.ServerID)
	if err != nil {
		logAction.SetError("Could not apply the saved sets", err.Error(), map[string]any{"server_id": req.ServerID})
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Sync = &state
	httpx.SendResponse(w, ld, response)
}

// GetMediaServerSync godoc
// @Summary      Get Media Server Sync Progress
// @Description  The progress of the latest time the saved sets were applied to a media server.
// @Tags         MediaServer
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=mediaServerSyncResponse}
// @Router       /api/mediaserver/sync [get]
func GetMediaServerSync(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	var response mediaServerSyncResponse
	if state, ok := mediaserver.GetMediaServerSync(); ok {
		response.Sync = &state
	}
	httpx.SendResponse(w, ld, response)
}

// StopMediaServerSync godoc
// @Summary      Stop Media Server Sync
// @Description  Stop applying the saved sets to a media server. The image being applied is finished first.
// @Tags         MediaServer
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=mediaServerSyncResponse}
// @Router       /api/mediaserver/sync [delete]
func StopMediaServerSync(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Stop Applying Saved Sets to Media Server", logging.LevelInfo)
	logAction.AppendResult("stopped", mediaserver.StopMediaServerSync())

	var response mediaServerSyncResponse
	if state, ok := mediaserver.GetMediaServerSync(); ok {
		response.Sync = &state
	}
	httpx.SendResponse(w, ld, response)
}
