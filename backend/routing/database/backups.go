package routes_db

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type databaseBackupsResponse struct {
	Backups []database.DatabaseBackup `json:"backups"` // Newest first
	Folder  string                    `json:"folder"`  // Where the backups are saved
	Keep    int                       `json:"keep"`    // How many backups are kept
}

type createDatabaseBackupResponse struct {
	Backup database.DatabaseBackup `json:"backup"` // The new backup
	databaseBackupsResponse
}

// GetDatabaseBackups godoc
// @Summary      Get Database Backups
// @Description  List the database backups in the backups folder, newest first.
// @Tags         Database
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=databaseBackupsResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db/backups [get]
func GetDatabaseBackups(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Database Backups", logging.LevelDebug)

	response, _ := listDatabaseBackups(logAction)
	httpx.SendResponse(w, ld, response)
}

// CreateDatabaseBackup godoc
// @Summary      Create Database Backup
// @Description  Back up the database now, then delete the oldest backups so only the number set in Backups.Keep are left.
// @Tags         Database
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=createDatabaseBackupResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db/backups [post]
func CreateDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Create Database Backup", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var response createDatabaseBackupResponse
	backup, Err := database.CreateBackup(ctx, config.Current.Backups.Keep)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Backup = backup
	response.databaseBackupsResponse, _ = listDatabaseBackups(logAction)
	httpx.SendResponse(w, ld, response)
}

// listDatabaseBackups returns the backups, and sets an error on the action if they can't be read
func listDatabaseBackups(logAction *logging.LogAction) (response databaseBackupsResponse, ok bool) {
	response = databaseBackupsResponse{Backups: []database.DatabaseBackup{}, Folder: database.BackupsFolder(), Keep: config.Current.Backups.Keep}
	backups, err := database.ListBackups()
	if err != nil {
		logAction.SetError("Could not read the backups folder", "Make sure the config folder can be read", map[string]any{"error": err.Error(), "folder": response.Folder})
		return response, false
	}
	response.Backups = backups
	return response, true
}
