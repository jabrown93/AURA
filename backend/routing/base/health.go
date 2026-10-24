package routes_base

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"net/http"
	"time"
)

type healthCheckResponse struct {
	Status      string `json:"status"`                 // ok, starting, setup or error
	AppVersion  string `json:"app_version"`            // The version of aura
	LoadingStep string `json:"loading_step,omitempty"` // What aura is doing while it starts
	Database    string `json:"database"`               // ok, not_open or error
}

// How long the database check can take before it fails
const healthDatabaseTimeout = 3 * time.Second

// HealthCheck godoc
// @Summary      Health Check
// @Description  Check that aura is running and its database can be read. Returns 500 if the database can't be read. Used by the Docker health check.
// @Tags         Health
// @Produce      json
// @Success      200  {object}  routes_base.healthCheckResponse
// @Failure      500  {object}  httpx.JSONResponse
// @Router       /api/ [get]
// @Router       /api/health [get]
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), "Health Check")
	logAction := ld.AddAction("Checking Health", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)

	response := healthCheckResponse{AppVersion: config.AppVersion}
	switch {
	case config.AppFullyLoaded:
		response.Status = "ok"
	case config.Loaded && !config.Valid:
		response.Status = "setup"
	default:
		response.Status = "starting"
		response.LoadingStep = config.AppLoadingStep
	}

	// The database is opened while aura starts
	if !database.IsOpen() {
		response.Database = "not_open"
		httpx.SendResponse(w, ld, response)
		return
	}
	checkCtx, cancel := context.WithTimeout(ctx, healthDatabaseTimeout)
	defer cancel()
	if err := database.Check(checkCtx); err != nil {
		response.Status, response.Database = "error", "error"
		logAction.SetError("The database can't be read", "Check that the config folder is on a local disk and has free space", map[string]any{"error": err.Error()})
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Health check failed: the database can't be read")
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Database = "ok"
	httpx.SendResponse(w, ld, response)
}
