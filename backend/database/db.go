package database

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"fmt"
)

const LATEST_DB_VERSION = 6

var Client DB

type SQliteDB struct {
	Config config.Config_Database
	conn   *sql.DB
}

type DB interface {
	// Open Database Connection
	GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo)

	// Initialize the database connection
	Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo)

	// Get Database Type
	GetConfig() (config config.Config_Database)

	// Create Main Tables
	CreateTables(ctx context.Context) (Err logging.LogErrorInfo)

	// Create VERSION table
	CreateVersionTable(ctx context.Context) (Err logging.LogErrorInfo)

	// Get the current database version (from VERSION table)
	GetCurrentVersion(ctx context.Context) (version int, Err logging.LogErrorInfo)

	// Update the VERSION table to newVersion
	UpdateVersionTable(ctx context.Context, newVersion int) (Err logging.LogErrorInfo)

	// Perform database vacuuming to optimize the database
	Vacuum(ctx context.Context) (Err logging.LogErrorInfo)

	// Create Auth table
	CreateAuthTable(ctx context.Context) (Err logging.LogErrorInfo)

	// Get Auth Token Secret
	GetAuthTokenSecret(ctx context.Context) (secret string, Err logging.LogErrorInfo)

	// Backup Database
	Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo)

	// Upsert Converted Saved Item
	UpsertSavedItem(ctx context.Context, newItem models.DBSavedItem) (Err logging.LogErrorInfo)

	// Check Media Item Exists
	CheckIfMediaItemExists(ctx context.Context, TMDB_ID, libraryTitle string) (ignored bool, ignoredMode string, sets []models.DBSavedSet, logErr logging.LogErrorInfo)

	// Get All Media Items
	GetAllMediaItems(ctx context.Context) (items []models.MediaItem, logErr logging.LogErrorInfo)

	// Get All Media Items with info on whether they have saved sets or are ignored
	GetAllMediaItemsWithFlags(ctx context.Context) (items []MediaItemWithFlags, logErr logging.LogErrorInfo)

	// Update Media Item
	UpdateMediaItem(ctx context.Context, updatedItem models.MediaItem) (Err logging.LogErrorInfo)

	// Delete Media Item and Ignored Item entries for a given TMDB ID and Library Title
	DeleteMediaItemAndIgnoredStatus(ctx context.Context, TMDB_ID, libraryTitle string) (Err logging.LogErrorInfo)

	// Get All Saved Sets
	GetAllSavedSets(ctx context.Context, dbFilter models.DBFilter) (out PagedSavedItems, logErr logging.LogErrorInfo)

	// Get All Unique Users from Saved Sets
	GetAllUniqueUsers(ctx context.Context) (users []string, logErr logging.LogErrorInfo)

	// Get Count of Saved Sets (Unique TMDB ID + Library Title combinations)
	//GetCountSavedSets(ctx context.Context) (count int, logErr logging.LogErrorInfo)

	// Delete Poster Set (and associated images) by ID
	DeletePosterSetForMediaItem(ctx context.Context, tmdbID, libraryTitle, setID string) (Err logging.LogErrorInfo)

	// Delete All Poster Sets for Media Item
	DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, libraryTitle string) (Err logging.LogErrorInfo)

	// Ignore Media Item
	IgnoreMediaItem(ctx context.Context, tmdbID, libraryTitle, mode, currentSets string) (Err logging.LogErrorInfo)

	// Stop Ignoring Media Item
	StopIgnoringMediaItem(ctx context.Context, TMDB_ID, libraryTitle string) (Err logging.LogErrorInfo)

	// Get Temp Ignored Items
	GetTempIgnoredItems(ctx context.Context) (items []models.MediaItem, Err logging.LogErrorInfo)

	// Update Media Item on_server flag
	UpdateMediaItemOnServer(ctx context.Context, tmdbID string, libraryTitle string, onServer bool) (logErr logging.LogErrorInfo)
}

func NewDatabaseClient() (DB, logging.LogErrorInfo) {
	dbConfig := config.Current.Database
	switch dbConfig.Type {
	case "sqlite3":
		return &SQliteDB{Config: dbConfig}, logging.LogErrorInfo{}
	default:
		return nil, logging.LogErrorInfo{
			Message: fmt.Sprintf("unsupported database type: %s", dbConfig.Type),
		}
	}
}

func BuildDSN() (string, logging.LogErrorInfo) {
	dbConfig := config.Current.Database
	switch dbConfig.Type {
	case "sqlite3":
		return dbConfig.Path, logging.LogErrorInfo{}
	// case "mysql":
	// 	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
	// 		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name), logging.LogErrorInfo{}
	// case "postgresql":
	// 	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
	// 		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name), logging.LogErrorInfo{}
	default:
		return "", logging.LogErrorInfo{
			Message: fmt.Sprintf("unsupported database type: %s", dbConfig.Type),
		}
	}
}

func GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo) {
	if Client == nil {
		return nil, false, logging.Error_DBClientNotInitialized()
	}
	return Client.GetDBConnection(ctx)
}

func Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo) {
	dbClient, err := NewDatabaseClient()
	if err.Message != "" {
		return false, err
	}
	Client = dbClient
	return dbClient.Init(ctx)
}

func GetConfig() (cfg config.Config_Database) {
	if Client == nil {
		return config.Current.Database
	}
	return Client.GetConfig()
}

func CreateVersionTable(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateVersionTable(ctx)
}

func GetCurrentVersion(ctx context.Context) (version int, Err logging.LogErrorInfo) {
	if Client == nil {
		return 0, logging.Error_DBClientNotInitialized()
	}
	return Client.GetCurrentVersion(ctx)
}

func UpdateVersionTable(ctx context.Context, newVersion int) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateVersionTable(ctx, newVersion)
}

func CreateAuthTable(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateAuthTable(ctx)
}

func GetAuthTokenSecret(ctx context.Context) (secret string, Err logging.LogErrorInfo) {
	if Client == nil {
		return "", logging.Error_DBClientNotInitialized()
	}
	return Client.GetAuthTokenSecret(ctx)
}

func Vacuum(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.Vacuum(ctx)
}

func CreateTables(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateTables(ctx)
}

func Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.Backup(ctx, currentVersion, newVersion)
}

func UpsertSavedItem(ctx context.Context, newItem models.DBSavedItem) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpsertSavedItem(ctx, newItem)
}

func CheckIfMediaItemExists(ctx context.Context, TMDB_ID, libraryTitle string) (ignored bool, ignoreMode string, sets []models.DBSavedSet, logErr logging.LogErrorInfo) {
	if Client == nil {
		return false, "", []models.DBSavedSet{}, logging.Error_DBClientNotInitialized()
	}
	return Client.CheckIfMediaItemExists(ctx, TMDB_ID, libraryTitle)
}

func GetAllMediaItems(ctx context.Context) (items []models.MediaItem, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []models.MediaItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllMediaItems(ctx)
}

func GetAllMediaItemsWithFlags(ctx context.Context) (items []MediaItemWithFlags, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []MediaItemWithFlags{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllMediaItemsWithFlags(ctx)
}

func UpdateMediaItem(ctx context.Context, updatedItem models.MediaItem) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateMediaItem(ctx, updatedItem)
}

func DeleteMediaItemAndIgnoredStatus(ctx context.Context, TMDB_ID, libraryTitle string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteMediaItemAndIgnoredStatus(ctx, TMDB_ID, libraryTitle)
}

func GetAllSavedSets(ctx context.Context, dbFilter models.DBFilter) (out PagedSavedItems, logErr logging.LogErrorInfo) {
	if Client == nil {
		return PagedSavedItems{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllSavedSets(ctx, dbFilter)
}

func GetAllUniqueUsers(ctx context.Context) (users []string, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []string{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllUniqueUsers(ctx)
}

// func GetCountSavedSets(ctx context.Context) (count int, logErr logging.LogErrorInfo) {
// 	if Client == nil {
// 		return 0, logging.Error_DBClientNotInitialized()
// 	}
// 	return Client.GetCountSavedSets(ctx)
// }

func DeletePosterSetForMediaItem(ctx context.Context, tmdbID, libraryTitle, setID string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeletePosterSetForMediaItem(ctx, tmdbID, libraryTitle, setID)
}

func DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, libraryTitle string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteAllPosterSetsForMediaItem(ctx, tmdbID, libraryTitle)
}

func IgnoreMediaItem(ctx context.Context, tmdbID, libraryTitle, mode, currentSets string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.IgnoreMediaItem(ctx, tmdbID, libraryTitle, mode, currentSets)
}

func StopIgnoringMediaItem(ctx context.Context, TMDB_ID, libraryTitle string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.StopIgnoringMediaItem(ctx, TMDB_ID, libraryTitle)
}

func GetTempIgnoredItems(ctx context.Context) (items []models.MediaItem, Err logging.LogErrorInfo) {
	if Client == nil {
		return []models.MediaItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetTempIgnoredItems(ctx)
}

func UpdateMediaItemOnServer(ctx context.Context, tmdbID string, libraryTitle string, onServer bool) (logErr logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateMediaItemOnServer(ctx, tmdbID, libraryTitle, onServer)
}
