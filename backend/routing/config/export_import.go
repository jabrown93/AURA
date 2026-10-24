package routes_config

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Largest config file that can be imported
const maxImportConfigSize = 1 << 20 // 1 MiB

// ExportAppConfig godoc
// @Summary      Export Config
// @Description  Download the full application configuration as a YAML file. With redacted=true, the secrets, emails and addresses are replaced so it can be shared; a redacted file can't be imported.
// @Tags         Config
// @Produce      application/x-yaml
// @Param        redacted  query     bool  false  "Replace the secrets, emails and addresses, for sharing"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {file}    file  "The config as YAML"
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/export [get]
func ExportAppConfig(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Export Config", logging.LevelInfo)

	exportConfig := config.Current.Clone()
	// The User ID is retrieved from the media server, so it is not exported
	exportConfig.MediaServer.UserID = ""

	data, err := yaml.Marshal(exportConfig)
	if err != nil {
		logAction.SetError("Failed to export config", err.Error(), nil)
		httpx.SendResponse(w, ld, nil)
		return
	}

	now := time.Now()
	header := fmt.Sprintf(`# aura configuration, exported %s (aura %s)
# This file contains passwords, tokens and API keys. Keep it private.
# To restore it, import it in Settings > Advanced, or place it in the config folder
# as config.yaml and start aura with AURA_IMPORT_CONFIG=true.

`, now.Format(time.RFC3339), config.AppVersion)
	filename := fmt.Sprintf("aura-config-%s.yaml", now.Format("2006-01-02"))

	redacted := r.URL.Query().Get("redacted") == "true"
	if redacted {
		data, err = config.RedactConfigYAML(data)
		if err != nil {
			logAction.SetError("Failed to redact config", err.Error(), nil)
			httpx.SendResponse(w, ld, nil)
			return
		}
		header = fmt.Sprintf(`%s, exported %s (aura %s)
# Passwords, tokens, API keys, emails and addresses are replaced, so it can be shared.
# Each address is replaced by host-1, host-2 and so on. It can't be imported.

`, config.RedactedConfigMarker, now.Format(time.RFC3339), config.AppVersion)
		filename = fmt.Sprintf("aura-config-redacted-%s.yaml", now.Format("2006-01-02"))
	}

	logAction.Complete()
	logging.LOGGER.Info().Timestamp().Bool("redacted", redacted).Msg("Config exported")

	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	// The file contains secrets, so it must not be cached
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(header))
	_, _ = w.Write(data)
}

type importConfigRequest struct {
	YAML string `json:"yaml"` // The config file contents in config.yaml format
}

// ImportAppConfig godoc
// @Summary      Import Config
// @Description  Replace the application configuration with a config file
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        config  body      importConfigRequest  true  "Config file contents"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200     {object}  httpx.JSONResponse{data=routes_config.updateConfigResponse}
// @Failure      500     {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/import [post]
func ImportAppConfig(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Import Config", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req importConfigRequest
	var response updateConfigResponse

	r.Body = http.MaxBytesReader(w, r.Body, maxImportConfigSize)
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Import Config")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	if strings.TrimSpace(req.YAML) == "" {
		logAction.SetError("The config file is empty", "Choose a config file exported from aura (config.yaml format)", nil)
		httpx.SendResponse(w, ld, response)
		return
	}
	// Importing a redacted file would replace the real secrets with <redacted>
	if strings.HasPrefix(strings.TrimSpace(req.YAML), config.RedactedConfigMarker) {
		logAction.SetError("This is a redacted config for sharing, it can't be imported", "Import a full export instead", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	newConfig, err := config.ParseConfigYAML([]byte(req.YAML))
	if err != nil {
		logAction.SetError("The config file could not be read", "Make sure the file is a valid aura config file (config.yaml format)", map[string]any{"error": err.Error()})
		httpx.SendResponse(w, ld, response)
		return
	}

	response = updateFullConfig(ctx, ld, logAction, newConfig, false)
	if ld.Status != logging.StatusError && logAction.Error == nil {
		logging.LOGGER.Info().Timestamp().Msg("Config imported")
	}
	httpx.SendResponse(w, ld, response)
}
