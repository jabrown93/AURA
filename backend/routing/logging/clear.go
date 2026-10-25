package routes_logging

import (
	"aura/logging"
	"aura/utils"
	"aura/utils/httpx"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
)

type ClearLogFiles_Response struct {
	Message string `json:"message"`
}

// ClearLogs godoc
// @Summary      Clear Logs
// @Description  Clear log files from the server. You can choose to clear the current log file or all old log files while keeping the current one. This endpoint is useful for maintenance and managing disk space used by logs.
// @Tags         Logging
// @Produce      json
// @Param        option  query     string  false  "Clear Option (current or old)" default(current)
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200     {object}  httpx.JSONResponse{data=ClearLogFiles_Response}
// @Failure      500     {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/logs [delete]
func ClearLogFiles(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Clear Logs", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response ClearLogFiles_Response

	// Get the query parameter to determine which logs to clear (current file or old files)
	clearOption := r.URL.Query().Get("option") // "current" or "old"

	if clearOption == "" {
		clearOption = "current" // Default to clearing current log file
	} else if clearOption != "current" && clearOption != "old" {
		logAction.SetError("Invalid clear option. Must be 'current' or 'old'.", "", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	// Check if the logs folder exists
	Err := utils.CreateFolderIfNotExists(ctx, logging.LogFolder)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Clear logs based on the option
	switch clearOption {
	case "current":
		// If the file exists, truncate it to clear its content
		if _, err := os.Stat(logging.LogFilePath); err == nil {
			f, err := os.OpenFile(logging.LogFilePath, os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				logAction.SetError("Failed to open log file for truncation", "Ensure the file is not read-only and you have permissions", map[string]any{"error": err.Error()})
				httpx.SendResponse(w, ld, response)
				return
			}
			// Ensure changes are flushed to disk
			if err := f.Sync(); err != nil {
				logAction.SetError("Failed to sync log file after truncation", "Ensure the file system is not read-only", map[string]any{"error": err.Error()})
				f.Close()
				httpx.SendResponse(w, ld, response)
				return
			}
			f.Close()
			response.Message = "Current log file cleared successfully"
		} else {
			ld.Status = logging.StatusWarn
			response.Message = "No current log file to clear"
		}
	case "old":
		// Delete all log files except the current one
		files, err := os.ReadDir(logging.LogFolder)
		if err != nil {
			logAction.SetError("Failed to read log directory", "Ensure the logs folder exists and is accessible", map[string]any{"error": err.Error()})
			httpx.SendResponse(w, ld, response)
			return
		}

		deletedFiles := 0
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".log") && file.Name() != "aura.log" {
				err := os.Remove(path.Join(logging.LogFolder, file.Name()))
				if err != nil {
					logAction.SetError(fmt.Sprintf("Failed to delete log file: %s", file.Name()), "Ensure the file is not in use and you have permissions", map[string]any{"error": err.Error()})
					httpx.SendResponse(w, ld, response)
					return
				}
				deletedFiles++
			}
		}
		if deletedFiles == 0 {
			ld.Status = logging.StatusWarn
			response.Message = "No old log files to delete"
		} else {
			response.Message = fmt.Sprintf("Cleared %d old log file(s) successfully", deletedFiles)
		}
	}

	httpx.SendResponse(w, ld, response)
}
