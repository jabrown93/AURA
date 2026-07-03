package routes_kometa

import (
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type importResponse struct {
	Message string               `json:"message"`
	Running bool                 `json:"running"`
	Result  *kometa.ImportResult `json:"result,omitempty"`
}

// TriggerKometaImport godoc
// @Summary      Trigger Kometa Asset Import
// @Description  Scan the configured Kometa asset directory, upload matching assets to Plex, and record them in the database. Runs asynchronously; poll GET /api/kometa/import for progress.
// @Tags         Kometa
// @Accept       json
// @Produce      json
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=routes_kometa.importResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/kometa/import [post]
func TriggerKometaImport(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Trigger Kometa Import", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response importResponse

	if !config.Current.Images.Kometa.Enabled || config.Current.MediaServer.Type != "Plex" {
		logAction.SetError("Kometa mode is not enabled", "Enable Kometa mode (Plex only) before importing assets", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	if started := kometa.StartImport(); !started {
		response.Running = true
		response.Message = "A Kometa import is already running"
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Running = true
	response.Message = "Kometa asset import started"
	httpx.SendResponse(w, ld, response)
}

// GetKometaImportStatus godoc
// @Summary      Get Kometa Asset Import Status
// @Description  Return whether a Kometa asset import is currently running and the result of the most recent run.
// @Tags         Kometa
// @Produce      json
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=routes_kometa.importResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/kometa/import [get]
func GetKometaImportStatus(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	ld.AddAction("Get Kometa Import Status", logging.LevelTrace)

	running, result := kometa.Status()
	response := importResponse{Running: running, Result: result}
	if running {
		response.Message = "A Kometa import is currently running"
	} else if result != nil {
		response.Message = "Last Kometa import result"
	} else {
		response.Message = "No Kometa import has run yet"
	}

	httpx.SendResponse(w, ld, response)
}
