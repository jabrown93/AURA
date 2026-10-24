package migration

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func migrate_1_to_2(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v1 to v2", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 1).Int("To Version", 2).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	// Create a backup of the current database
	backupErr := database.Backup(ctx, 1, 2)
	if backupErr.Message != "" {
		return backupErr
	}

	// Get DB connection
	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	// Rename tables to *Backup1
	for _, tableName := range []string{"SavedItems", "MediaItems", "PosterSets"} {
		if renameErr := v1_2_RenameTable(ctx, conn, tableName); renameErr.Message != "" {
			return renameErr
		}
	}

	// Create new tables: MediaItems, Movies, Series, Seasons, Episodes, PosterSets, ImageFiles, SavedItems
	createTablesErr := database.CreateTables(ctx)
	if createTablesErr.Message != "" {
		return createTablesErr
	}

	// Create Auth table
	Err = database.CreateAuthTable(ctx)
	if Err.Message != "" {
		return Err
	}

	// Convert the old data to the new format
	convertOldDataErr := v1_2_ConvertOldData(ctx, conn)
	if convertOldDataErr.Message != "" {
		return convertOldDataErr
	}

	// Drop backup tables
	for _, tableName := range []string{"SavedItemsBackup1", "MediaItemsBackup1", "PosterSetsBackup1"} {
		dropQuery := fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName)
		_, err := conn.ExecContext(ctx, dropQuery)
		if err != nil {
			logAction.SetError(fmt.Sprintf("Failed to drop backup table %s", tableName), "", map[string]any{
				"error": err.Error(),
			})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v1.0 to v2.0 completed successfully")
	return logging.LogErrorInfo{}
}

func v1_2_RenameTable(ctx context.Context, conn *sql.DB, tableName string) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Renaming %s Table", tableName), logging.LevelTrace)
	defer logAction.Complete()

	query := fmt.Sprintf(`
		ALTER TABLE %s RENAME TO %sBackup1;
	`, tableName, tableName)
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError(fmt.Sprintf("Failed to rename %s table", tableName), "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

type v1MediaItemRow struct {
	TMDB_ID      string
	LibraryTitle string
	RatingKey    sql.NullString
	ItemType     sql.NullString
	Title        sql.NullString
	Year         sql.NullInt64
	MovieJSON    sql.NullString
	SeriesJSON   sql.NullString
}

type v1PosterSetRow struct {
	PosterSetID    string
	TMDB_ID        string
	LibraryTitle   string
	PosterSetUser  sql.NullString
	PosterSetJSON  sql.NullString
	LastDownloaded sql.NullString
	SelectedTypes  sql.NullString
	AutoDownload   sql.NullBool
}

func v1_2_ConvertOldData(ctx context.Context, conn *sql.DB) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Converting Old Data to New Format", logging.LevelInfo)
	defer logAction.Complete()

	// Cache: Add all media server sections and items
	success := mediaserver.GetAllLibrarySectionsAndItems(ctx, true)
	if !success {
		logAction.SetError("Failed to fetch all library sections and items from media server", "", nil)
		return *logAction.Error
	}

	// First we will do a query on SavedItemsBackup1 to get all rows
	// SavedItemsBackup1 has columns: tmdb_id, library_title, poster_set_id
	query := `
		SELECT TMDB_ID, LibraryTitle, PosterSetID
		FROM SavedItemsBackup1;
	`
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to query SavedItemsBackup1 table", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}
	defer rows.Close()

	var newItems []models.DBSavedItem

	// Get total row count for progress tracking
	var totalRows int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM SavedItemsBackup1;
	`).Scan(&totalRows)
	if err != nil {
		logAction.SetError("Failed to get row count from SavedItemsBackup1", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}

	currentItemIndex := 0

	ignoredKeys := map[string]struct{}{}

	// For each row in SavedItemsBackup1, we will fetch the corresponding MediaItem and PosterSet
	for rows.Next() {

		itemTMDB_ID := ""
		itemLibraryTitle := ""
		posterSetID := ""

		var newItem models.DBSavedItem

		if err := rows.Scan(&itemTMDB_ID, &itemLibraryTitle, &posterSetID); err != nil {
			addToWarningFile(2, logging.LogErrorInfo{
				Message: "Failed to scan row from SavedItemsBackup1",
				Detail: map[string]any{
					"error": err.Error(),
				},
			})
			continue
		}

		currentItemIndex++
		logging.LOGGER.Trace().Timestamp().Int("current_item", currentItemIndex).Int("total_items", totalRows).
			Str("TMDB_ID", itemTMDB_ID).Str("LibraryTitle", itemLibraryTitle).
			Msg("Processing Item for migration")

		isIgnored := v1_2_isLegacyIgnorePosterSetID(posterSetID)
		if isIgnored {
			ignoredKeys[itemTMDB_ID+"|"+itemLibraryTitle] = struct{}{}
		}

		// Since we have the TMDB_ID and LibraryTitle, we can get the MediaItem from the MediaItemsBackup1 table
		// If we get that info, and Movie or Series is empty, we will get the full info from the Media Server
		failedToGetFromDB := false
		var mediaItemRow v1MediaItemRow
		queryMediaItem := `
				SELECT TMDB_ID, LibraryTitle, RatingKey, Type, Title, Year, Movie_JSON, Series_JSON
				FROM MediaItemsBackup1
				WHERE TMDB_ID = ? AND LibraryTitle = ?;
				`
		err := conn.QueryRowContext(ctx, queryMediaItem, itemTMDB_ID, itemLibraryTitle).Scan(
			&mediaItemRow.TMDB_ID,
			&mediaItemRow.LibraryTitle,
			&mediaItemRow.RatingKey,
			&mediaItemRow.ItemType,
			&mediaItemRow.Title,
			&mediaItemRow.Year,
			&mediaItemRow.MovieJSON,
			&mediaItemRow.SeriesJSON,
		)
		if err != nil || (!mediaItemRow.ItemType.Valid || (mediaItemRow.ItemType.String == "movie" && !mediaItemRow.MovieJSON.Valid) || (mediaItemRow.ItemType.String == "show" && !mediaItemRow.SeriesJSON.Valid)) {
			failedToGetFromDB = true
		} else {
			newItem.MediaItem = models.MediaItem{
				TMDB_ID:      mediaItemRow.TMDB_ID,
				LibraryTitle: mediaItemRow.LibraryTitle,
				RatingKey:    mediaItemRow.RatingKey.String,
				Type:         mediaItemRow.ItemType.String,
				Title:        mediaItemRow.Title.String,
				Year:         int(mediaItemRow.Year.Int64),
			}
			if mediaItemRow.ItemType.String == "movie" && mediaItemRow.MovieJSON.Valid {
				var movie models.MediaItemMovie
				err := json.Unmarshal([]byte(mediaItemRow.MovieJSON.String), &movie)
				if err != nil {
					addToWarningFile(2, logging.LogErrorInfo{
						Message: "Failed to unmarshal Movie JSON from MediaItemsBackup1 during migration",
						Detail: map[string]any{
							"TMDB_ID":      itemTMDB_ID,
							"LibraryTitle": itemLibraryTitle,
							"error":        err.Error(),
						},
					})
					continue
				}
				newItem.MediaItem.Movie = &movie
				if movie.File.Path == "" {
					failedToGetFromDB = true
				}
			} else if mediaItemRow.ItemType.String == "show" && mediaItemRow.SeriesJSON.Valid {
				failedToGetFromDB = true
			}
		}

		if failedToGetFromDB {
			// Here we can try to get the MediaItem from the cache as a fallback
			cachedItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(itemLibraryTitle, itemTMDB_ID)
			if !found || cachedItem == nil {
				logging.LOGGER.Warn().Timestamp().Str("TMDB_ID", itemTMDB_ID).Str("LibraryTitle", itemLibraryTitle).
					Msg("MediaItem not found in cache either during migration")
				addToWarningFile(2, logging.LogErrorInfo{
					Message: "MediaItem not found in database or cache during migration",
					Detail: map[string]any{
						"TMDB_ID":      itemTMDB_ID,
						"LibraryTitle": itemLibraryTitle,
					},
				})
				continue
			}

			if cachedItem.Type != "movie" {
				// Get the full item info from the media server
				foundInServer, getItemErr := mediaserver.GetMediaItemDetails(ctx, cachedItem)
				if getItemErr.Message != "" {
					addToWarningFile(2, getItemErr)
					continue
				}
				if !foundInServer {
					addToWarningFile(2, logging.LogErrorInfo{
						Message: "MediaItem not found on media server during migration",
						Detail: map[string]any{
							"TMDB_ID":      itemTMDB_ID,
							"LibraryTitle": itemLibraryTitle,
						},
					})
					continue
				}
				newItem.MediaItem = *cachedItem
			}
		}

		// Now we can fetch the each PosterSet from PosterSetsBackup1 with the given TMDB_ID and LibraryTitle
		if !isIgnored {
			queryPosterSets := `
				SELECT PosterSetID, TMDB_ID, LibraryTitle, PosterSetUser, PosterSet_JSON, LastDownloaded, SelectedTypes, AutoDownload
				FROM PosterSetsBackup1
				WHERE TMDB_ID = ? AND LibraryTitle = ?;
				`
			psRows, err := conn.QueryContext(ctx, queryPosterSets, itemTMDB_ID, itemLibraryTitle)
			if err != nil {
				addToWarningFile(2, logging.LogErrorInfo{
					Message: "Failed to query PosterSetsBackup1 table",
					Detail: map[string]any{
						"error":        err.Error(),
						"TMDB_ID":      itemTMDB_ID,
						"LibraryTitle": itemLibraryTitle,
					},
				})
				continue
			}
			for psRows.Next() {
				var psRow v1PosterSetRow
				if err := psRows.Scan(
					&psRow.PosterSetID,
					&psRow.TMDB_ID,
					&psRow.LibraryTitle,
					&psRow.PosterSetUser,
					&psRow.PosterSetJSON,
					&psRow.LastDownloaded,
					&psRow.SelectedTypes,
					&psRow.AutoDownload,
				); err != nil {
					addToWarningFile(2, logging.LogErrorInfo{
						Message: "Failed to scan row from PosterSetsBackup1",
						Detail: map[string]any{
							"error":        err.Error(),
							"TMDB_ID":      itemTMDB_ID,
							"LibraryTitle": itemLibraryTitle,
						},
					})
					continue
				}

				if v1_2_isLegacyIgnorePosterSetID(psRow.PosterSetID) {
					continue
				}

				var posterSet models.PosterSet
				if psRow.PosterSetJSON.Valid {
					if err := json.Unmarshal([]byte(psRow.PosterSetJSON.String), &posterSet); err != nil {
						addToWarningFile(2, logging.LogErrorInfo{
							Message: "Failed to unmarshal PosterSet JSON",
							Detail: map[string]any{
								"error":        err.Error(),
								"TMDB_ID":      itemTMDB_ID,
								"LibraryTitle": itemLibraryTitle,
							},
						})
						continue
					}

				}

				posterSet.ID = psRow.PosterSetID

				posterSet.Type, posterSet.Title, posterSet.UserCreated, posterSet.DateCreated, posterSet.DateUpdated = v1_2_extractPosterSetBaseInfo(psRow.PosterSetJSON.String)
				if posterSet.Type == "" {
					logging.LOGGER.Warn().Timestamp().Str("PosterSetID", posterSet.ID).
						Msg("PosterSet Type is empty after extraction during migration")
				}

				// Extract Poster Images from JSON
				posterSet.Images = v1_2_extractPosterImages(newItem.MediaItem.TMDB_ID, psRow.PosterSetJSON.String)

				newItem.PosterSets = append(newItem.PosterSets, models.DBPosterSetDetail{
					PosterSet:      posterSet,
					LastDownloaded: utils.ConvertDateStringToTime(psRow.LastDownloaded.String),
					SelectedTypes:  v1_2_parseSelectedTypes(psRow.SelectedTypes.String),
					AutoDownload:   psRow.AutoDownload.Bool,
				})
				logging.LOGGER.Trace().Timestamp().Str("PosterSetID", posterSet.ID).Int("num_images", len(posterSet.Images)).
					Msg("Extracted PosterSet and images for migration")
			}
			if err := psRows.Err(); err != nil {
				addToWarningFile(2, logging.LogErrorInfo{
					Message: "Failed while iterating over PosterSetsBackup1 rows",
					Detail: map[string]any{
						"error":        err.Error(),
						"TMDB_ID":      itemTMDB_ID,
						"LibraryTitle": itemLibraryTitle,
					},
				})
			}
			psRows.Close()
		}
		newItems = append(newItems, newItem)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while iterating over SavedItemsBackup1 rows", "", map[string]any{
			"error": err.Error(),
		})
		return *logAction.Error
	}
	rows.Close()

	// Now we have all the newItems converted, we can upsert them into the new tables
	for _, newItem := range newItems {
		upsertErr := database.UpsertSavedItem(ctx, newItem)
		if upsertErr.Message != "" {
			addToWarningFile(2, upsertErr)
			continue
		}

		if _, ok := ignoredKeys[newItem.MediaItem.TMDB_ID+"|"+newItem.MediaItem.LibraryTitle]; ok {
			if errInfo := v1_2_insertIgnoredItemTemp(ctx, conn, newItem.MediaItem.TMDB_ID, newItem.MediaItem.LibraryTitle); errInfo.Message != "" {
				addToWarningFile(2, errInfo)
				continue
			}
		}
	}

	return logging.LogErrorInfo{}
}

func v1_2_extractPosterSetBaseInfo(ps string) (psType, psTitle, psUser string, psDateCreated, psDateUpdated time.Time) {
	type imageFile struct {
		ID string `json:"ID"`
	}

	var psData struct {
		Type  string `json:"Type"`
		Title string `json:"Title"`
		User  struct {
			Name string `json:"Name"`
		}
		DateCreated  string      `json:"DateCreated"`
		DateUpdated  string      `json:"DateUpdated"`
		OtherPosters []imageFile `json:"OtherPosters"`
	}

	if err := json.Unmarshal([]byte(ps), &psData); err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to unmarshal PosterSet JSON for extracting PosterSet Type")
		return "", "", "", time.Time{}, time.Time{}
	}

	if psData.Type == "movie" && len(psData.OtherPosters) > 0 {
		psData.Type = "collection"
	}

	return psData.Type, psData.Title, psData.User.Name, utils.ConvertDateStringToTime(psData.DateCreated), utils.ConvertDateStringToTime(psData.DateUpdated)
}

func v1_2_extractPosterImages(itemTMDBID string, ps string) []models.ImageFile {
	out := make([]models.ImageFile, 0, 32)

	type imageFile struct {
		ID       string `json:"ID"`
		Type     string `json:"Type"`
		Modified string `json:"Modified"`
		FileSize int64  `json:"FileSize"`
		Src      string `json:"Src"`
		Blurhash string `json:"Blurhash"`

		Movie *struct {
			ID string `json:"ID"`
		} `json:"Movie,omitempty"`

		// Optional, for show sets (if present in payload)
		Show *struct {
			ID string `json:"ID"`
		} `json:"Show,omitempty"`

		Season *struct {
			Number int `json:"Number"`
		} `json:"Season,omitempty"`
		Episode *struct {
			Title         string `json:"Title"`
			EpisodeNumber int    `json:"EpisodeNumber"`
			SeasonNumber  int    `json:"SeasonNumber"`
		} `json:"Episode,omitempty"`
	}

	var psData struct {
		Type           string      `json:"Type"`
		Poster         imageFile   `json:"Poster"`
		Backdrop       imageFile   `json:"Backdrop"`
		SeasonPosters  []imageFile `json:"SeasonPosters"`
		TitleCards     []imageFile `json:"TitleCards"`
		OtherPosters   []imageFile `json:"OtherPosters"`
		OtherBackdrops []imageFile `json:"OtherBackdrops"`
	}

	if err := json.Unmarshal([]byte(ps), &psData); err != nil {
		logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to unmarshal PosterSet JSON for extracting PosterImages")
		return out
	}

	// For "movie" poster sets, only include images that belong to this movie (by nested Movie.ID).
	// For "show" sets, poster/backdrop typically belong to the show itself (no nested Movie), so include.
	belongsToItem := func(img imageFile) bool {
		if strings.TrimSpace(itemTMDBID) == "" {
			return false
		}

		switch strings.ToLower(strings.TrimSpace(psData.Type)) {
		case "movie", "collection":
			if img.Movie == nil {
				return false
			}
			return strings.TrimSpace(img.Movie.ID) == strings.TrimSpace(itemTMDBID)

		case "show":
			// If Show.ID exists, enforce it; otherwise assume poster/backdrop belong to the show.
			if img.Show != nil && strings.TrimSpace(img.Show.ID) != "" {
				return strings.TrimSpace(img.Show.ID) == strings.TrimSpace(itemTMDBID)
			}
			return true

		default:
			// safest default: require explicit match if present
			if img.Movie != nil && strings.TrimSpace(img.Movie.ID) != "" {
				return strings.TrimSpace(img.Movie.ID) == strings.TrimSpace(itemTMDBID)
			}
			if img.Show != nil && strings.TrimSpace(img.Show.ID) != "" {
				return strings.TrimSpace(img.Show.ID) == strings.TrimSpace(itemTMDBID)
			}
			return false
		}
	}

	addImage := func(img imageFile, forcedType string) {
		if img.ID == "" {
			return
		}
		if !belongsToItem(img) {
			return
		}

		imgFile := models.ImageFile{
			ID:       img.ID,
			Type:     forcedType,
			Modified: utils.ConvertDateStringToTime(img.Modified),
			FileSize: img.FileSize,
			Src:      img.Src,
			Blurhash: img.Blurhash,
		}
		out = append(out, imgFile)
	}

	// Poster/Backdrop for this specific item
	addImage(psData.Poster, "poster")
	addImage(psData.Backdrop, "backdrop")

	// Other posters/backdrops for this specific item
	for _, op := range psData.OtherPosters {
		addImage(op, "poster")
	}
	for _, ob := range psData.OtherBackdrops {
		addImage(ob, "backdrop")
	}

	// Show-only: Season posters / Titlecards (these don’t belong to “other shows” in a movie collection set)
	if len(psData.SeasonPosters) > 0 {
		for _, sp := range psData.SeasonPosters {
			if sp.Season == nil {
				logging.LOGGER.Warn().Timestamp().Str("ImageID", sp.ID).Msg("SeasonPoster missing Season info during migration")
				continue
			}
			imgFile := models.ImageFile{
				ID:           sp.ID,
				Type:         "season_poster",
				Modified:     utils.ConvertDateStringToTime(sp.Modified),
				FileSize:     sp.FileSize,
				Src:          sp.Src,
				Blurhash:     sp.Blurhash,
				SeasonNumber: &sp.Season.Number,
			}
			out = append(out, imgFile)
		}
	}
	if len(psData.TitleCards) > 0 {
		for _, tc := range psData.TitleCards {
			if tc.Episode == nil {
				logging.LOGGER.Warn().Timestamp().Str("ImageID", tc.ID).Msg("TitleCard missing Episode info during migration")
				continue
			}
			imgFile := models.ImageFile{
				ID:            tc.ID,
				Type:          "titlecard",
				Modified:      utils.ConvertDateStringToTime(tc.Modified),
				FileSize:      tc.FileSize,
				Src:           tc.Src,
				Blurhash:      tc.Blurhash,
				Title:         tc.Episode.Title,
				SeasonNumber:  &tc.Episode.SeasonNumber,
				EpisodeNumber: &tc.Episode.EpisodeNumber,
			}
			out = append(out, imgFile)
		}
	}

	return out
}

func v1_2_parseSelectedTypes(selectedTypesStr string) models.SelectedTypes {
	// Selected Types are a comma-separated string
	// Sometimes there may be no types selected, resulting in an empty string
	// Sometimes there may be only one type selected, resulting in no commas
	if selectedTypesStr == "" {
		return models.SelectedTypes{
			Poster:              false,
			Backdrop:            false,
			SeasonPoster:        false,
			SpecialSeasonPoster: false,
			Titlecard:           false,
		}
	}
	types := strings.Split(selectedTypesStr, ",")
	selectedTypes := models.SelectedTypes{}
	for _, t := range types {
		switch strings.TrimSpace(t) {
		case "poster":
			selectedTypes.Poster = true
		case "backdrop":
			selectedTypes.Backdrop = true
		case "season_poster", "seasonPoster":
			selectedTypes.SeasonPoster = true
		case "special_season_poster", "specialSeasonPoster":
			selectedTypes.SpecialSeasonPoster = true
		case "titlecard":
			selectedTypes.Titlecard = true
		}
	}
	return selectedTypes
}

func v1_2_isLegacyIgnorePosterSetID(id string) bool {
	id = strings.TrimSpace(strings.ToLower(id))
	return id == "ignore" || id == "ignore_always" || id == "ignore_temp"
}

func v1_2_insertIgnoredItemTemp(ctx context.Context, conn *sql.DB, tmdbID, libraryTitle string) logging.LogErrorInfo {
	// FK requires MediaItems row to exist, so call this AFTER UpsertSavedItem succeeds.
	_, err := conn.ExecContext(ctx, `
        INSERT INTO IgnoredItems (tmdb_id, library_title, mode)
        VALUES (?, ?, 'temp')
        ON CONFLICT(tmdb_id, library_title) DO UPDATE SET
            mode = 'temp';
    `, tmdbID, libraryTitle)
	if err != nil {
		return logging.LogErrorInfo{
			Message: "Failed to insert ignored item during migration",
			Detail: map[string]any{
				"error":        err.Error(),
				"TMDB_ID":      tmdbID,
				"LibraryTitle": libraryTitle,
			},
		}
	}
	return logging.LogErrorInfo{}
}
