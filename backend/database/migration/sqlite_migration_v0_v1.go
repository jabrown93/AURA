package migration

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

func migrate_0_to_1(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v0 to v1", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 0).Int("To Version", 1).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	// Create a backup of the current database
	backupErr := database.Backup(ctx, 0, 1)
	if backupErr.Message != "" {
		return backupErr
	}

	// Get DB connection
	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	// Rename SavedItems table to SavedItemsBackup
	renameSavedItemsTableErr := v0_1_RenameSavedItemsTable(ctx, conn)
	if renameSavedItemsTableErr.Message != "" {
		return renameSavedItemsTableErr
	}

	// Create new tables: VERSION, MediaItems, PosterSets, SavedItems
	createTablesErr := v0_1_CreateTables(ctx, conn)
	if createTablesErr.Message != "" {
		return createTablesErr
	}

	// Convert old SavedItems data to new format
	convertOldSavedItemsErr := v0_1_ConvertOldSavedItems(ctx, conn)
	if convertOldSavedItemsErr.Message != "" {
		return convertOldSavedItemsErr
	}

	// Drop the old SavedItemsBackup table
	dropOldTableQuery := `DROP TABLE IF EXISTS SavedItemsBackup;`
	_, err := conn.Exec(dropOldTableQuery)
	if err != nil {
		return logging.LogErrorInfo{
			Message: "Failed to drop old SavedItemsBackup table after migration",
			Detail: map[string]any{
				"error": err.Error(),
			},
		}
	}

	return Err
}

func v0_1_CreateTables(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {

	database.CreateVersionTable(ctx)

	// Create MediaItems table
	createMediaItemsTableErr := v0_1_CreateMediaItemsTable(ctx, conn)
	if createMediaItemsTableErr.Message != "" {
		return createMediaItemsTableErr
	}

	// Create PosterSets table
	createPosterSetsTableErr := v0_1_CreatePosterSetsTable(ctx, conn)
	if createPosterSetsTableErr.Message != "" {
		return createPosterSetsTableErr
	}

	// Create SavedItems table
	createSavedItemsTableErr := v0_1_CreateSavedItemsTable(ctx, conn)
	if createSavedItemsTableErr.Message != "" {
		return createSavedItemsTableErr
	}

	return logging.LogErrorInfo{}
}

func v0_1_CreateMediaItemsTable(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating MediaItems Table", logging.LevelDebug)
	defer logAction.Complete()

	query := `
	CREATE TABLE IF NOT EXISTS MediaItems (
    TMDB_ID TEXT NOT NULL,
    LibraryTitle TEXT NOT NULL,

    RatingKey TEXT,
    Type TEXT,
    Title TEXT,
    Year INTEGER,
    Movie_JSON JSON,        -- Store Movie struct as JSON string
    Series_JSON JSON,       -- Store Series struct as JSON string
    

	Thumb TEXT,
    ContentRating TEXT,
    Summary TEXT,
    UpdatedAt INTEGER,
    AddedAt INTEGER,
    ReleasedAt INTEGER,
    Guids_JSON JSON,		-- Store Guids as JSON string
	Full_JSON JSON,         -- Store full MediaItem as JSON string

    PRIMARY KEY (TMDB_ID, LibraryTitle)
);`
	_, err := conn.Exec(query)
	if err != nil {
		logAction.SetError("Failed to create MediaItems table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func v0_1_CreatePosterSetsTable(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating PosterSets Table", logging.LevelDebug)
	defer logAction.Complete()

	query := `
	CREATE TABLE IF NOT EXISTS PosterSets (
	PosterSetID TEXT NOT NULL,
	TMDB_ID TEXT NOT NULL,
	LibraryTitle TEXT NOT NULL,

	PosterSetUser TEXT,
	PosterSet_JSON JSON,

	LastDownloaded DATETIME,
	SelectedTypes TEXT,
	AutoDownload BOOLEAN,
	
	PRIMARY KEY (PosterSetID, TMDB_ID, LibraryTitle),
	FOREIGN KEY (TMDB_ID, LibraryTitle) REFERENCES MediaItem(TMDB_ID, LibraryTitle)
	);`
	_, err := conn.Exec(query)
	if err != nil {
		logAction.SetError("Failed to create PosterSets table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func v0_1_CreateSavedItemsTable(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating SavedItems Table", logging.LevelDebug)
	defer logAction.Complete()

	query := `
	CREATE TABLE IF NOT EXISTS SavedItems (
	TMDB_ID TEXT NOT NULL,
	LibraryTitle TEXT NOT NULL,
    PosterSetID TEXT NOT NULL,
    PRIMARY KEY (TMDB_ID, LibraryTitle, PosterSetID),
    FOREIGN KEY (TMDB_ID, LibraryTitle) REFERENCES MediaItem(TMDB_ID, LibraryTitle),
    FOREIGN KEY (PosterSetID) REFERENCES PosterSet(PosterSetID)
);
`
	_, err := conn.Exec(query)
	if err != nil {
		logAction.SetError("Failed to create SavedItems table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func v0_1_RenameSavedItemsTable(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Renaming SavedItems Table", logging.LevelDebug)
	defer logAction.Complete()

	query := `
	ALTER TABLE SavedItems RENAME TO SavedItemsBackup0;
	`
	_, err := conn.Exec(query)
	if err != nil {
		logAction.SetError("Failed to rename SavedItems table", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func v0_1_MediaItemJSONToMediaItem(mediaItemJSON string) (v0_1_MediaItem, bool) {
	// Unmarshal media item JSON
	var mediaItem v0_1_MediaItem
	err := json.Unmarshal([]byte(mediaItemJSON), &mediaItem)
	if err != nil {
		addToWarningFile(1, logging.LogErrorInfo{
			Message: "Failed to unmarshal MediaItem JSON during migration",
			Detail: map[string]any{
				"error": err.Error(),
				"json":  mediaItemJSON,
			},
		})
		return v0_1_MediaItem{}, false
	}

	// Get the TMDB ID from the Guids
	tmdbID := ""
	for _, guid := range mediaItem.Guids {
		if guid.Provider == "tmdb" {
			tmdbID = guid.ID
			break
		}
	}
	mediaItem.TMDB_ID = tmdbID

	if mediaItem.Type != "movie" && mediaItem.Type != "show" {
		mediaItem.Type = ""
	}
	return mediaItem, true
}

func v0_1_PosterSetJSONToPosterSet(posterSetJSON string) (v0_1_PosterSet, bool) {
	// Unmarshal poster set JSON
	var posterSet v0_1_PosterSet
	err := json.Unmarshal([]byte(posterSetJSON), &posterSet)
	if err != nil {
		addToWarningFile(1, logging.LogErrorInfo{
			Message: "Failed to unmarshal PosterSet JSON during migration",
			Detail: map[string]any{
				"error": err.Error(),
				"json":  posterSetJSON,
			},
		})
		return posterSet, false
	}

	// Check PosterSet ID
	if posterSet.ID == "" {
		addToWarningFile(1, logging.LogErrorInfo{
			Message: "PosterSet ID is empty during migration",
			Detail:  map[string]any{"posterSetJSON": posterSetJSON},
		})
		return posterSet, false
	}

	return posterSet, true
}

type v0_1_PosterSet struct {
	ID                string            `json:"ID"`
	Title             string            `json:"Title"`
	Type              string            `json:"Type"`
	User              mediux.SetUser    `json:"User"`
	DateCreated       time.Time         `json:"DateCreated"`
	DateUpdated       time.Time         `json:"DateUpdated"`
	Poster            *v0_1_PosterFile  `json:"Poster,omitempty"`
	OtherPosters      []v0_1_PosterFile `json:"OtherPosters,omitempty"` // User for movie collections
	Backdrop          *v0_1_PosterFile  `json:"Backdrop,omitempty"`
	OtherBackdrops    []v0_1_PosterFile `json:"OtherBackdrops,omitempty"` // User for movie collections
	SeasonPosters     []v0_1_PosterFile `json:"SeasonPosters,omitempty"`
	TitleCards        []v0_1_PosterFile `json:"TitleCards,omitempty"`
	Status            string            `json:"Status,omitempty"`
	TMDB_PosterPath   string            `json:"TMDB_PosterPath,omitempty"`
	TMDB_BackdropPath string            `json:"TMDB_BackdropPath,omitempty"`
}

type v0_1_PosterFile struct {
	ID       string                 `json:"ID"`
	Type     string                 `json:"Type"`
	Modified time.Time              `json:"Modified"`
	FileSize int64                  `json:"FileSize"`
	Src      string                 `json:"Src"`
	Blurhash string                 `json:"Blurhash"`
	Movie    *v0_1_PosterFileMovie  `json:"Movie,omitempty"`
	Show     *v0_1_PosterFileShow   `json:"Show,omitempty"`
	Season   *v0_1_ImageFileSeason  `json:"Season,omitempty"`
	Episode  *v0_1_ImageFileEpisode `json:"Episode,omitempty"`
}

type v0_1_ImageFileSeason struct {
	Number int `json:"Number"`
}

type v0_1_ImageFileEpisode struct {
	SeasonNumber  int `json:"SeasonNumber"`
	EpisodeNumber int `json:"EpisodeNumber"`
}

type v0_1_PosterFileShow struct {
	ID        string           `json:"ID"`
	Title     string           `json:"Title"`
	MediaItem models.MediaItem `json:"MediaItem"`
}
type v0_1_PosterFileMovie struct {
	ID          string           `json:"ID"`
	Title       string           `json:"Title"`
	Status      string           `json:"Status,omitempty"`
	Tagline     string           `json:"Tagline,omitempty"`
	Slug        string           `json:"Slug,omitempty"`
	DateUpdated time.Time        `json:"DateUpdated"`
	TvdbID      string           `json:"TvdbID,omitempty"`
	ImdbID      string           `json:"ImdbID,omitempty"`
	TraktID     string           `json:"TraktID,omitempty"`
	ReleaseDate string           `json:"ReleaseDate,omitempty"`
	MediaItem   models.MediaItem `json:"MediaItem"`
}

type v0_1_MediaItem struct {
	TMDB_ID         string                  `json:"TMDB_ID"`                 // TMDB ID of the media item
	LibraryTitle    string                  `json:"LibraryTitle"`            // Title of the library/section the item belongs to
	RatingKey       string                  `json:"RatingKey"`               // RatingKey is the internal ID from the media server
	Type            string                  `json:"Type"`                    // "movie" or "show"
	Title           string                  `json:"Title"`                   // Title of the media item
	Year            int                     `json:"Year"`                    // Release year of the media item
	ExistInDatabase bool                    `json:"ExistInDatabase"`         // Indicates if the item exists in the local database
	DBSavedSets     []models.DBSavedSet     `json:"DBSavedSets,omitempty"`   // Poster sets saved in the database for this item
	Thumb           string                  `json:"Thumb,omitempty"`         // URL to the thumbnail image
	ContentRating   string                  `json:"ContentRating,omitempty"` // Content rating (e.g., "PG-13")
	Summary         string                  `json:"Summary,omitempty"`       // Summary or description of the media item
	UpdatedAt       int64                   `json:"UpdatedAt,omitempty"`     // Last updated timestamp
	AddedAt         int64                   `json:"AddedAt,omitempty"`       // Added timestamp
	ReleasedAt      int64                   `json:"ReleasedAt,omitempty"`    // Release date timestamp
	Guids           []models.MediaItemGuid  `json:"Guids,omitempty"`         // List of GUIDs from different providers
	Movie           *models.MediaItemMovie  `json:"Movie,omitempty"`         // Present if Type is "movie"; Contains file info
	Series          *models.MediaItemSeries `json:"Series,omitempty"`        // Present if Type is "show"; Contains seasons and episodes info
}

type v0_1_DBPosterSetDetail struct {
	PosterSetID    string         `json:"PosterSetID"`
	PosterSet      v0_1_PosterSet `json:"PosterSet"`
	PosterSetJSON  string         `json:"PosterSetJSON"`
	LastDownloaded string         `json:"LastDownloaded"`
	SelectedTypes  []string       `json:"SelectedTypes"`
	AutoDownload   bool           `json:"AutoDownload"`
	ToDelete       bool           `json:"ToDelete"` // Flag to indicate if the poster set should be deleted (Not used in DB)
}

// MediaItemWithPosterSets groups a media item with its poster sets.
type v0_1_DBMediaItemWithPosterSets struct {
	TMDB_ID       string                   `json:"TMDB_ID"`
	LibraryTitle  string                   `json:"LibraryTitle"`
	MediaItem     v0_1_MediaItem           `json:"MediaItem"`
	MediaItemJSON string                   `json:"MediaItemJSON"`
	PosterSets    []v0_1_DBPosterSetDetail `json:"PosterSets"`
}

func convertCurrentMediaItemToV0_1(mediaItem models.MediaItem) v0_1_MediaItem {
	return v0_1_MediaItem{
		TMDB_ID:         mediaItem.TMDB_ID,
		LibraryTitle:    mediaItem.LibraryTitle,
		RatingKey:       mediaItem.RatingKey,
		Type:            mediaItem.Type,
		Title:           mediaItem.Title,
		Year:            mediaItem.Year,
		ExistInDatabase: len(mediaItem.DBSavedSets) > 0,
		DBSavedSets:     mediaItem.DBSavedSets,
		Thumb:           "",
		ContentRating:   mediaItem.ContentRating,
		Summary:         mediaItem.Summary,
		UpdatedAt:       mediaItem.UpdatedAt,
		AddedAt:         mediaItem.AddedAt,
		ReleasedAt:      mediaItem.ReleasedAt,
		Guids:           mediaItem.Guids,
		Movie:           mediaItem.Movie,
		Series:          mediaItem.Series,
	}
}

func insertAllInfoIntoTables(ctx context.Context, conn *sql.DB, item v0_1_DBMediaItemWithPosterSets) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Adding Item into Database", logging.LevelInfo)
	defer logAction.Complete()

	// Return for dev-testing
	//time.Sleep(1 * time.Second)
	// return logging.LogErrorInfo{}
	// ld := logAction.AddSubAction("Simulate Error", logging.LevelTrace)
	// ld.SetError("Simulated db error for testing", "This is only a test", map[string]any{
	// 	"item": item,
	// })
	// return *ld.Error

	// Start a DB transaction
	actionBeginTx := logAction.AddSubAction("Start DB Transaction", logging.LevelTrace)
	tx, err := conn.Begin()
	if err != nil {
		actionBeginTx.SetError("Failed to begin DB transaction", err.Error(), map[string]any{
			"item": item,
		})
		return *actionBeginTx.Error
	}
	actionBeginTx.Complete()

	// Remove any "ignore" poster sets for this TMDB_ID and LibraryTitle from the database
	actionRemoveIgnorePosterSets := logAction.AddSubAction("Remove 'Ignore' From Poster Sets Table", logging.LevelTrace)
	_, err = tx.Exec(`
        DELETE FROM PosterSets
        WHERE PosterSetID = ? AND TMDB_ID = ? AND LibraryTitle = ?
    `, "ignore", item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
	if err != nil {
		tx.Rollback()
		actionRemoveIgnorePosterSets.SetError("Failed to remove 'ignore' poster sets", err.Error(), map[string]any{
			"item": item,
		})
		return *actionRemoveIgnorePosterSets.Error
	}
	actionRemoveIgnorePosterSets.Complete()

	// Remove any ignore entries from the SavedItems table
	actionRemoveIgnoreEntries := logAction.AddSubAction("Remove 'Ignore' Entries From Saved Items Table", logging.LevelTrace)
	_, err = tx.Exec(`
		DELETE FROM SavedItems
		WHERE PosterSetID = ? AND TMDB_ID = ? AND LibraryTitle = ?
	`, "ignore", item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
	if err != nil {
		tx.Rollback()
		actionRemoveIgnoreEntries.SetError("Failed to remove 'ignore' entries from SavedItems", err.Error(), map[string]any{
			"item": item,
		})
		return *actionRemoveIgnoreEntries.Error
	}
	actionRemoveIgnoreEntries.Complete()

	// Check if the media item already exists
	isUpdate := false
	var exists int
	actionCheckExistence := logAction.AddSubAction("Check If Media Item Exists in DB", logging.LevelTrace)
	err = tx.QueryRow(`
    SELECT COUNT(*) FROM MediaItems WHERE TMDB_ID = ? AND LibraryTitle = ?
`, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle).Scan(&exists)
	if err != nil {
		tx.Rollback()
		actionCheckExistence.SetError("Failed to check if media item exists", err.Error(), map[string]any{
			"item": item,
		})
		return *actionCheckExistence.Error
	}
	if exists > 0 {
		isUpdate = true
	}
	actionCheckExistence.Complete()

	// Insert into MediaItems table
	mediaItemJSON, _ := json.Marshal(item.MediaItem)
	guidsJSON, _ := json.Marshal(item.MediaItem.Guids)
	movieJSON, _ := json.Marshal(item.MediaItem.Movie)
	seriesJSON, _ := json.Marshal(item.MediaItem.Series)
	actionAddToMediaItemsTable := logAction.AddSubAction("Insert/Update Media Item in MediaItems Table", logging.LevelTrace)
	_, err = tx.Exec(`
    INSERT INTO MediaItems (
        TMDB_ID, LibraryTitle, RatingKey, Type, Title, Year, Thumb, ContentRating, Summary, UpdatedAt, AddedAt, ReleasedAt, Guids_JSON, Movie_JSON, Series_JSON, Full_JSON
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(TMDB_ID, LibraryTitle) DO UPDATE SET
        RatingKey = excluded.RatingKey,
        Type = excluded.Type,
        Title = excluded.Title,
        Year = excluded.Year,
        Thumb = excluded.Thumb,
        ContentRating = excluded.ContentRating,
        Summary = excluded.Summary,
        UpdatedAt = excluded.UpdatedAt,
        AddedAt = excluded.AddedAt,
        ReleasedAt = excluded.ReleasedAt,
        Guids_JSON = excluded.Guids_JSON,
        Movie_JSON = excluded.Movie_JSON,
        Series_JSON = excluded.Series_JSON,
        Full_JSON = excluded.Full_JSON
`, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle, item.MediaItem.RatingKey,
		item.MediaItem.Type, item.MediaItem.Title, item.MediaItem.Year, item.MediaItem.Thumb,
		item.MediaItem.ContentRating, item.MediaItem.Summary, item.MediaItem.UpdatedAt, item.MediaItem.AddedAt,
		item.MediaItem.ReleasedAt, string(guidsJSON), string(movieJSON), string(seriesJSON), string(mediaItemJSON))
	if err != nil {
		tx.Rollback()
		actionAddToMediaItemsTable.SetError("Failed to insert/update media item", err.Error(), map[string]any{
			"item": item,
		})
		return *actionAddToMediaItemsTable.Error
	}
	actionAddToMediaItemsTable.Complete()

	// Find the other PosterSet Items for this TMDB_ID and LibraryTitle
	// We need to do this to remove any Selected Types from this set
	// If there is none left, then delete that entry entirely
	actionFindExistingPosterSets := logAction.AddSubAction("Find Existing Poster Sets for Media Item", logging.LevelTrace)
	rows, err := tx.Query(`
		SELECT PosterSetID, LibraryTitle, SelectedTypes FROM PosterSets
		WHERE TMDB_ID = ? AND LibraryTitle = ?
	`, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
	if err != nil {
		tx.Rollback()
		actionFindExistingPosterSets.SetError("Failed to query existing poster sets", err.Error(), map[string]any{
			"item": item,
		})
		return *actionFindExistingPosterSets.Error
	}
	defer rows.Close()
	actionFindExistingPosterSets.Complete()

	existingPosterSets := make(map[string][]string) // Map of PosterSetID to SelectedTypes
	for rows.Next() {
		var posterSetID string
		var selectedTypesStr string
		if err := rows.Scan(&posterSetID, &item.MediaItem.LibraryTitle, &selectedTypesStr); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to scan existing poster sets", err.Error(), map[string]any{
				"item": item,
			})
			return *logAction.Error
		}
		selectedTypes := strings.Split(selectedTypesStr, ",")
		existingPosterSets[posterSetID] = selectedTypes
	}
	if err := rows.Err(); err != nil {
		tx.Rollback()
		logAction.SetError("Failed to iterate over existing poster sets", err.Error(), map[string]any{
			"item": item,
		})
		return *logAction.Error
	}

	// Remove SelectedTypes that are no longer present in the new PosterSets
	for existingID, existingTypes := range existingPosterSets {
		found := false
		for _, newPosterSet := range item.PosterSets {
			if newPosterSet.PosterSet.ID == existingID {
				found = true
				break
			}
		}
		if !found {
			// Remove the SelectedTypes from this PosterSet
			newTypes := []string{}
			for _, t := range existingTypes {
				keep := true
				for _, newPosterSet := range item.PosterSets {
					if slices.Contains(newPosterSet.SelectedTypes, t) {
						keep = false
					}
					if !keep {
						break
					}
				}
				if keep {
					newTypes = append(newTypes, t)
				}
			}
			if len(newTypes) == 0 {
				// No SelectedTypes left, delete the PosterSet entry
				actionDeletePosterSet := logAction.AddSubAction("Delete Poster Set with No SelectedTypes", logging.LevelTrace)
				_, err = tx.Exec(`
					DELETE FROM PosterSets
					WHERE PosterSetID = ? AND TMDB_ID = ? AND LibraryTitle = ?
				`, existingID, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
				if err != nil {
					tx.Rollback()
					actionDeletePosterSet.SetError("Failed to delete poster set with no selected types", err.Error(), map[string]any{
						"item": item,
					})
					return *actionDeletePosterSet.Error
				}
				actionDeletePosterSet.Complete()
				actionDeletePosterSet.AppendResult("action", "deleted poster set, no selected types left")
				actionDeletePosterSet.AppendResult("poster_set_id", existingID)
				actionDeletePosterSet.AppendResult("tmdb_id", item.MediaItem.TMDB_ID)
				actionDeletePosterSet.AppendResult("library_title", item.MediaItem.LibraryTitle)
				actionDeletePosterSet.AppendResult("title", item.MediaItem.Title)
				actionDeletePosterSet.AppendResult("selected_types", existingTypes)

				// Delete from SavedItems table as well
				actionDeleteSavedItem := logAction.AddSubAction("Delete Saved Item for Poster Set with No SelectedTypes", logging.LevelTrace)
				_, err = tx.Exec(`
					DELETE FROM SavedItems
					WHERE PosterSetID = ? AND TMDB_ID = ? AND LibraryTitle = ?
				`, existingID, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
				if err != nil {
					tx.Rollback()
					actionDeleteSavedItem.SetError("Failed to delete saved item for poster set with no selected types", err.Error(), map[string]any{
						"item": item,
					})
					return *actionDeleteSavedItem.Error
				}
				actionDeleteSavedItem.Complete()
				actionDeleteSavedItem.AppendResult("action", "deleted saved item, no selected types left")
				actionDeleteSavedItem.AppendResult("poster_set_id", existingID)
				actionDeleteSavedItem.AppendResult("title", item.MediaItem.Title)
				actionDeleteSavedItem.AppendResult("tmdb_id", item.MediaItem.TMDB_ID)
				actionDeleteSavedItem.AppendResult("library_title", item.MediaItem.LibraryTitle)

			} else {
				// Update the PosterSet with the new SelectedTypes
				actionUpdatePosterSet := logAction.AddSubAction("Update Poster Set with New SelectedTypes", logging.LevelTrace)
				_, err = tx.Exec(`
					UPDATE PosterSets
					SET SelectedTypes = ?
					WHERE PosterSetID = ? AND TMDB_ID = ? AND LibraryTitle = ?
				`, strings.Join(newTypes, ","), existingID, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle)
				if err != nil {
					tx.Rollback()
					actionUpdatePosterSet.SetError("Failed to update poster set with new selected types", err.Error(), map[string]any{
						"item": item,
					})
					return *actionUpdatePosterSet.Error
				}
				actionUpdatePosterSet.Complete()
				actionUpdatePosterSet.AppendResult("action", "updated poster set with new selected types")
				actionUpdatePosterSet.AppendResult("poster_set_id", existingID)
				actionUpdatePosterSet.AppendResult("tmdb_id", item.MediaItem.TMDB_ID)
				actionUpdatePosterSet.AppendResult("library_title", item.MediaItem.LibraryTitle)
				actionUpdatePosterSet.AppendResult("title", item.MediaItem.Title)
				actionUpdatePosterSet.AppendResult("selected_types", newTypes)
			}
		}
	}

	// Insert each of the PosterSet items
	for _, posterSet := range item.PosterSets {
		if posterSet.ToDelete {
			// Skip poster sets marked for deletion
			continue
		}
		posterSetJSON, _ := json.Marshal(posterSet.PosterSet)
		now := time.Now().UTC().Format(time.RFC3339)
		actionAddToPosterSetsTable := logAction.AddSubAction("Insert/Update Poster Set in PosterSets Table", logging.LevelTrace)
		_, err = tx.Exec(`
        INSERT INTO PosterSets (
            PosterSetID, TMDB_ID, LibraryTitle, PosterSetUser, PosterSet_JSON, LastDownloaded, SelectedTypes, AutoDownload
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(PosterSetID, TMDB_ID, LibraryTitle) DO UPDATE SET
            TMDB_ID = excluded.TMDB_ID,
            LibraryTitle = excluded.LibraryTitle,
            PosterSetUser = excluded.PosterSetUser,
            PosterSet_JSON = excluded.PosterSet_JSON,
            LastDownloaded = excluded.LastDownloaded,
            SelectedTypes = excluded.SelectedTypes,
            AutoDownload = excluded.AutoDownload;
    `, posterSet.PosterSet.ID, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle,
			posterSet.PosterSet.User.Username, string(posterSetJSON),
			now, strings.Join(posterSet.SelectedTypes, ","), posterSet.AutoDownload)
		if err != nil {
			tx.Rollback()
			actionAddToPosterSetsTable.SetError("Failed to insert poster set", err.Error(), map[string]any{
				"item": item,
			})
			return *actionAddToPosterSetsTable.Error
		}
		actionAddToPosterSetsTable.Complete()

		// Insert into SavedItems table
		actionAddToSavedItemsTable := logAction.AddSubAction("Insert Saved Item for Poster Set", logging.LevelTrace)
		_, err = tx.Exec(`
		INSERT OR IGNORE INTO SavedItems (TMDB_ID, LibraryTitle, PosterSetID) VALUES (?, ?, ?)
	`, item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle, posterSet.PosterSet.ID)
		if err != nil {
			tx.Rollback()
			actionAddToSavedItemsTable.SetError("Failed to insert saved item", err.Error(), map[string]any{
				"item": item,
			})
			return *actionAddToSavedItemsTable.Error
		}
		actionAddToSavedItemsTable.Complete()
	}

	// Commit the transaction
	actionCommitTx := logAction.AddSubAction("Commit DB Transaction", logging.LevelTrace)
	if err = tx.Commit(); err != nil {
		tx.Rollback()
		actionCommitTx.SetError("Failed to commit DB transaction", err.Error(), map[string]any{
			"item": item,
		})
		return *actionCommitTx.Error
	}
	actionCommitTx.Complete()

	transactionType := "inserted"
	if isUpdate {
		transactionType = "updated"
	}
	logAction.AppendResult("transaction_type", transactionType)
	logAction.AppendResult("title", item.MediaItem.Title)
	logAction.AppendResult("tmdb_id", item.MediaItem.TMDB_ID)
	logAction.AppendResult("library", item.MediaItem.LibraryTitle)

	return logging.LogErrorInfo{}
}

func v0_1_ConvertOldSavedItems(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Converting Old SavedItems Data", logging.LevelInfo)
	defer logAction.Complete()

	// Fetch all entries from the old SavedItemsBackup table
	fetchAllQuery := `SELECT media_item_id, media_item, poster_set_id, poster_set, selected_types, auto_download, last_update FROM SavedItemsBackup;`
	rows, err := conn.Query(fetchAllQuery)
	if err != nil {
		logAction.SetError("Failed to fetch data from SavedItemsBackup", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}
	defer rows.Close()

	// Read all rows into a slice
	type OldSavedItem struct {
		MediaItemID   string
		MediaItemJSON string
		PosterSetID   string
		PosterSetJSON string
		SelectedTypes string
		AutoDownload  bool
		LastUpdate    string
	}
	var items []OldSavedItem
	results := map[string]int{}

	results["SavedItemFails"] = 0
	results["SavedItemSuccess"] = 0

	for rows.Next() {
		var item OldSavedItem
		err := rows.Scan(&item.MediaItemID, &item.MediaItemJSON, &item.PosterSetID, &item.PosterSetJSON, &item.SelectedTypes, &item.AutoDownload, &item.LastUpdate)
		if err != nil {
			results["SavedItemFails"]++
			addToWarningFile(1, logging.LogErrorInfo{
				Message: "Failed to scan row from old SavedItemsBackup table",
				Detail:  map[string]any{"error": err.Error()},
			})
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed to read data from SavedItemsBackup", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}
	rows.Close()

	for _, item := range items {
		logging.LOGGER.Debug().Timestamp().Str("Media Item ID", item.MediaItemID).Str("Poster Set ID", item.PosterSetID).Msg("Migration (0-1): Processing Saved Item from DB")

		// Get MediaItem struct from JSON
		mediaItem, mediaItemPassed := v0_1_MediaItemJSONToMediaItem(item.MediaItemJSON)
		if !mediaItemPassed {
			logging.LOGGER.Warn().Timestamp().Str("Media Item ID", item.MediaItemID).Msg("Migration (0-1): Skipping Saved Item due to invalid MediaItem JSON")
			results["SavedItemFails"]++
			continue
		}

		// Get PosterSet struct from JSON
		posterSet, posterSetPassed := v0_1_PosterSetJSONToPosterSet(item.PosterSetJSON)
		if !posterSetPassed {
			logging.LOGGER.Warn().Timestamp().Str("Poster Set ID", item.PosterSetID).Msg("Migration (0-1): Skipping Saved Item due to invalid PosterSet JSON")
			results["SavedItemFails"]++
			continue
		}

		// If the Media Item Type is empty, try to get it from the PosterSet
		if mediaItem.Type == "" {
			if posterSet.Poster != nil {
				if posterSet.Poster.Movie != nil {
					mediaItem.Type = "movie"
				} else if posterSet.Poster.Show != nil {
					mediaItem.Type = "show"
				}
			}
			if mediaItem.Type == "" && posterSet.Backdrop != nil {
				if posterSet.Backdrop.Movie != nil {
					mediaItem.Type = "movie"
				} else if posterSet.Backdrop.Show != nil {
					mediaItem.Type = "show"
				}
			}
			if mediaItem.Type == "" && len(posterSet.SeasonPosters) > 0 {
				for _, seasonPoster := range posterSet.SeasonPosters {
					if seasonPoster.Show != nil {
						mediaItem.Type = "show"
						break
					}
				}
			}
			if mediaItem.Type == "" && len(posterSet.TitleCards) > 0 {
				for _, titleCard := range posterSet.TitleCards {
					if titleCard.Show != nil {
						mediaItem.Type = "show"
						break
					}
				}
			}
		}

		// If we still don't have a valid type, skip this item
		if mediaItem.Type != "movie" && mediaItem.Type != "show" {
			results["SavedItemFails"]++
			addToWarningFile(1, logging.LogErrorInfo{
				Message: "MediaItem type is invalid during migration",
				Help:    fmt.Sprintf("Type must be 'movie' or 'show', got '%s'", mediaItem.Type),
				Detail: map[string]any{
					"MediaItem": mediaItem,
					"PosterSet": posterSet,
				},
			})
			continue
		}

		partialFound := 0
		if mediaItem.Type == "movie" && (mediaItem.Movie == nil || mediaItem.Movie.File.Path == "") {
			if posterSet.Poster != nil && posterSet.Poster.Movie != nil {
				movie := posterSet.Poster.Movie
				if mediaItem.LibraryTitle == "" && movie.MediaItem.LibraryTitle != "" {
					mediaItem.LibraryTitle = movie.MediaItem.LibraryTitle
					partialFound = 1
				}
				if mediaItem.TMDB_ID == "" && movie.ID != "" {
					mediaItem.TMDB_ID = movie.ID
					partialFound = 1
				}
			}
			if posterSet.Backdrop != nil && posterSet.Backdrop.Movie != nil {
				movie := posterSet.Backdrop.Movie
				if mediaItem.LibraryTitle == "" && movie.MediaItem.LibraryTitle != "" {
					mediaItem.LibraryTitle = movie.MediaItem.LibraryTitle
					partialFound = 1
				}
				if mediaItem.TMDB_ID == "" && movie.ID != "" {
					mediaItem.TMDB_ID = movie.ID
					partialFound = 1
				}
			}
		} else if mediaItem.Type == "show" && (mediaItem.Series == nil) {
			// Try to get missing info from PosterSet's Poster or Backdrop
			if posterSet.Poster != nil && posterSet.Poster.Show != nil {
				show := posterSet.Poster.Show
				if mediaItem.LibraryTitle == "" && show.MediaItem.LibraryTitle != "" {
					mediaItem.LibraryTitle = show.MediaItem.LibraryTitle
					partialFound = 1
				}
				if mediaItem.TMDB_ID == "" && show.ID != "" {
					mediaItem.TMDB_ID = show.ID
					partialFound = 1
				}
			}
			if posterSet.Backdrop != nil && posterSet.Backdrop.Show != nil {
				show := posterSet.Backdrop.Show
				if mediaItem.LibraryTitle == "" && show.MediaItem.LibraryTitle != "" {
					mediaItem.LibraryTitle = show.MediaItem.LibraryTitle
					partialFound = 1
				}
				if mediaItem.TMDB_ID == "" && show.ID != "" {
					mediaItem.TMDB_ID = show.ID
					partialFound = 1
				}
			}
			if mediaItem.TMDB_ID == "" || mediaItem.LibraryTitle == "" {
				for _, seasonPoster := range posterSet.SeasonPosters {
					if seasonPoster.Show != nil {
						if mediaItem.LibraryTitle == "" && seasonPoster.Show.MediaItem.LibraryTitle != "" {
							mediaItem.LibraryTitle = seasonPoster.Show.MediaItem.LibraryTitle
							partialFound = 1
						}
						if mediaItem.TMDB_ID == "" && seasonPoster.Show.ID != "" {
							mediaItem.TMDB_ID = seasonPoster.Show.ID
							partialFound = 1
						}
					}
				}
			}
			if mediaItem.TMDB_ID == "" || mediaItem.LibraryTitle == "" {
				for _, titleCard := range posterSet.TitleCards {
					if titleCard.Show != nil {
						if mediaItem.LibraryTitle == "" && titleCard.Show.MediaItem.LibraryTitle != "" {
							mediaItem.LibraryTitle = titleCard.Show.MediaItem.LibraryTitle
							partialFound = 1
						}
						if mediaItem.TMDB_ID == "" && titleCard.Show.ID != "" {
							mediaItem.TMDB_ID = titleCard.Show.ID
							partialFound = 1
						}
					}
				}
			}
		}

		// Check we have the minimum required information
		if mediaItem.TMDB_ID == "" || mediaItem.LibraryTitle == "" || partialFound == 1 {
			logging.LOGGER.Debug().Timestamp().Str("Media Item ID", item.MediaItemID).Str("TMDB ID", mediaItem.TMDB_ID).Msg("Migration (0-1): Trying to get missing Media Item info from cache")
			// Try and get the media item from the cache
			mediaItemFromCache, exist := cache.LibraryStore.GetMediaItemFromSectionByTitleAndYear(mediaItem.LibraryTitle, mediaItem.Title, mediaItem.Year)
			if exist {
				mediaItem = convertCurrentMediaItemToV0_1(*mediaItemFromCache)
			} else {
				results["SavedItemFails"]++
				addToWarningFile(1, logging.LogErrorInfo{
					Message: "Insufficient MediaItem data during migration and not found in cache",
					Help:    fmt.Sprintf("Title: '%s', TMDB_ID: '%s', LibraryTitle: '%s', Year: '%d'", mediaItem.Title, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Year),
					Detail: map[string]any{
						"MediaItem": mediaItem,
						"PosterSet": posterSet,
					},
				})
				continue
			}
		}

		// Create a v0_1_DBMediaItemWithPosterSets struct
		dbItem := v0_1_DBMediaItemWithPosterSets{
			TMDB_ID:      mediaItem.TMDB_ID,
			LibraryTitle: mediaItem.LibraryTitle,
			MediaItem:    mediaItem,
			PosterSets: []v0_1_DBPosterSetDetail{
				{
					PosterSetID:    posterSet.ID,
					PosterSet:      posterSet,
					LastDownloaded: item.LastUpdate,
					SelectedTypes:  strings.Split(item.SelectedTypes, ","), // Convert string to []string
					AutoDownload:   item.AutoDownload,
				},
			},
		}

		// Insert all info into the new tables
		insertErr := insertAllInfoIntoTables(ctx, conn, dbItem)
		if insertErr.Message != "" {
			results["SavedItemFails"]++
			addToWarningFile(1, logging.LogErrorInfo{
				Message: "Failed to insert migrated SavedItem into new tables",
				Detail:  insertErr.Detail,
			})
			continue
		}

		results["SavedItemSuccess"]++
	}

	// Convert results to map[string]any for assignment
	resultAny := make(map[string]any, len(results))
	for k, v := range results {
		resultAny[k] = v
	}
	logAction.Result = resultAny
	return logging.LogErrorInfo{}
}
