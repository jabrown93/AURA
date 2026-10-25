package silo

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Silo) GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Full Info for %s", utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	found = false
	Err = logging.LogErrorInfo{}

	// Get the Media Item details from Silo
	siloResp, Err := getCatalogItemDetail(ctx, s.Config, item.RatingKey, "")
	if Err.Message != "" {
		// If the item was not found, Silo may have changed its Content ID
		// Search the library for the item using its TMDB ID
		if !isNotFoundError(Err) || item.TMDB_ID == "" {
			logAction.SetErrorFromInfo(Err)
			return found, *logAction.Error
		}
		contentID, searchErr := findContentIDByTMDBID(ctx, s.Config, item)
		if searchErr.Message != "" {
			logAction.SetErrorFromInfo(searchErr)
			return found, *logAction.Error
		}
		logAction.AppendResult("content_id_changed", map[string]any{"old": item.RatingKey, "new": contentID})
		siloResp, Err = getCatalogItemDetail(ctx, s.Config, contentID, "")
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return found, *logAction.Error
		}
	}

	item.RatingKey = siloResp.ContentID
	item.Type = map[string]string{
		"movie":  "movie",
		"series": "show",
	}[siloResp.Type]
	item.Title = siloResp.Title
	item.Year = siloResp.Year
	item.ContentRating = siloResp.ContentRating
	item.Summary = siloResp.Overview

	// Movies have a release date, Series have a first air date
	releaseDate := siloResp.ReleaseDate
	if releaseDate == "" {
		releaseDate = siloResp.FirstAirDate
	}
	if t, err := time.Parse("2006-01-02", releaseDate); err == nil {
		item.ReleasedAt = t.Unix()
	}

	// Extract GUIDs and Ratings from the response
	item.Guids = getGUIDsAndRatingsFromResponse(siloResp)
	item.TMDB_ID = siloResp.TMDB_ID

	// If the item has a user rating from Silo, set the user rating
	// Silo uses whole stars from 1 to 5, so convert it to a scale of 10 to match Plex
	if siloResp.UserRating > 0 {
		item.Guids = append(item.Guids, models.MediaItemGuid{
			Provider: "user",
			Rating:   strconv.Itoa(siloResp.UserRating * 2),
		})
	}

	// If no TMDB ID found, get the value from MediUX using the GUID[tvdb]
	if item.TMDB_ID == "" && siloResp.TVDB_ID != "" {
		tmdbID, found, Err := mediux.SearchTMDBIDByTVDBID(ctx, siloResp.TVDB_ID, item.Type)
		if Err.Message != "" {
			logAction.AppendWarning("search_tmdb_id_error", "Failed to search TMDB ID from MediUX")
		}
		if found {
			item.TMDB_ID = tmdbID
		}
	}
	if item.TMDB_ID == "" {
		logAction.SetError("No TMDB ID found for the media item", "Ensure the media item has a valid TMDB ID in Silo",
			map[string]any{
				"rating_key":     item.RatingKey,
				"library_title":  item.LibraryTitle,
				"title":          item.Title,
				"provider_guids": item.Guids,
			})
		return found, *logAction.Error
	}

	// Depending on the Type, populate Movie or Series info
	switch item.Type {
	case "movie":
		if len(siloResp.Versions) > 0 {
			// Silo has one item for every library it is in, so use the file in this item's library
			version := siloResp.Versions[0]
			for _, v := range siloResp.Versions {
				if isPathInLibrary(s.Config, v.FilePath, item.LibraryTitle) {
					version = v
					break
				}
			}
			item.Movie = &models.MediaItemMovie{
				File: models.MediaItemFile{
					Path:     version.FilePath,
					Size:     version.FileSize,
					Duration: version.Duration * 1000,
				},
			}
			if item.AddedAt == 0 && !version.AddedAt.IsZero() {
				item.AddedAt = version.AddedAt.Unix()
			}
		}
	case "show":
		// For shows, fetch seasons and episodes
		Err = fetchSeasonsAndEpisodesForShow(ctx, s.Config, item)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return found, *logAction.Error
		}
		// Silo counts seasons that have no episodes in the library, so only count the seasons that were returned
		item.Series.SeasonCount = len(item.Series.Seasons)
		if len(siloResp.FolderPaths) > 0 {
			// Silo has one item for every library it is in, so use the folder in this item's library
			item.Series.Location = siloResp.FolderPaths[0]
			for _, folderPath := range siloResp.FolderPaths {
				if isPathInLibrary(s.Config, folderPath, item.LibraryTitle) {
					item.Series.Location = folderPath
					break
				}
			}
		}
		item.Series.EpisodeCount = 0
		for _, season := range item.Series.Seasons {
			item.Series.EpisodeCount += len(season.Episodes)
		}
	}

	// Check if Media Item exists in DB
	ignored, ignoredMode, sets, logErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition)
	if logErr.Message != "" {
		logAction.AppendWarning("message", "Failed to check if media item exists in database")
		logAction.AppendWarning("error", logErr)
	}
	if !ignored {
		item.DBSavedSets = sets
		item.IgnoredInDB = false
		item.IgnoredMode = ""
	} else {
		item.DBSavedSets = []models.DBSavedSet{}
		item.IgnoredInDB = true
		item.IgnoredMode = ignoredMode
	}

	// Check if Media Item exists in MediUX with a set
	if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
		item.HasMediuxSets = true
	}

	// Only the main media server's items go in the main library cache and the DB
	if s.Config.IsMain() {
		cache.LibraryStore.UpdateMediaItem(item.LibraryTitle, item)
		updateErr := database.UpdateMediaItemOnServer(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, true)
		if updateErr.Message != "" {
			logAction.AppendWarning("update_on_server_error", updateErr.Message)
		}
	}

	// Mark as found
	found = true
	return found, logging.LogErrorInfo{}
}

func getCatalogItemDetail(ctx context.Context, msConfig config.Config_MediaServer, contentID string, imageSize string) (siloResp SiloCatalogItemDetailResponse, Err logging.LogErrorInfo) {
	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "catalog", "items", contentID)
	if err != nil {
		message, help, detail := logging.Error_BaseUrlParsing(err)
		return siloResp, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
	}
	if imageSize != "" {
		query := u.Query()
		query.Set("image_size", imageSize)
		u.RawQuery = query.Encode()
	}

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return siloResp, Err
	}

	// Decode the Response
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Media Item Details Response")
	if Err.Message != "" {
		return siloResp, Err
	}

	// Save the item info so it is not retrieved again
	saveItemInfo(ctx, siloResp)

	return siloResp, logging.LogErrorInfo{}
}

func findContentIDByTMDBID(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem) (contentID string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Searching for %s by TMDB ID", utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "catalog")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return "", *logAction.Error
	}
	query := u.Query()
	query.Set("q", item.Title)
	query.Set("limit", "20")
	switch item.Type {
	case "movie":
		query.Set("type", "movie")
	case "show":
		query.Set("type", "series")
	}
	if section, found := libraryStore(msConfig).GetSectionByTitle(item.LibraryTitle); found && section.ID != "" {
		query.Set("library_id", section.ID)
	}
	u.RawQuery = query.Encode()

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return "", *logAction.Error
	}

	// Decode the Response
	var siloResp SiloCatalogResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Catalog Search Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return "", *logAction.Error
	}

	// Find the search result with a matching TMDB ID
	for _, catalogItem := range siloResp.Items {
		itemInfo, Err := getItemInfo(ctx, msConfig, catalogItem.ContentID)
		if Err.Message != "" {
			logAction.AppendWarning(fmt.Sprintf("skipped '%s'", catalogItem.ContentID), Err.Message)
			continue
		}
		if itemInfo.TMDB_ID == item.TMDB_ID {
			return catalogItem.ContentID, logging.LogErrorInfo{}
		}
	}

	logAction.SetError("Media item not found on Silo server", "The item may have been removed from the Silo server",
		map[string]any{
			"rating_key":    item.RatingKey,
			"tmdb_id":       item.TMDB_ID,
			"title":         item.Title,
			"library_title": item.LibraryTitle,
		})
	return "", *logAction.Error
}

func getGUIDsAndRatingsFromResponse(siloResp SiloCatalogItemDetailResponse) (guids []models.MediaItemGuid) {
	guids = []models.MediaItemGuid{}

	// First, add the provider IDs
	if siloResp.TMDB_ID != "" {
		guids = append(guids, models.MediaItemGuid{Provider: "tmdb", ID: siloResp.TMDB_ID})
	}
	if siloResp.IMDB_ID != "" {
		guids = append(guids, models.MediaItemGuid{Provider: "imdb", ID: siloResp.IMDB_ID})
	}
	if siloResp.TVDB_ID != "" {
		guids = append(guids, models.MediaItemGuid{Provider: "tvdb", ID: siloResp.TVDB_ID})
	}

	// Next, add the Ratings to the matching provider
	// Silo scores are 0-100, so convert them to a 0-10 scale
	for _, siloRating := range siloResp.Ratings {
		provider := map[string]string{
			"tmdb":        "tmdb",
			"imdb":        "imdb",
			"rt_critic":   "rottentomatoes",
			"rt_audience": "rottentomatoes",
		}[siloRating.Source]
		if provider == "" {
			continue
		}
		rating := strconv.FormatFloat(siloRating.Score/10, 'f', 1, 64)

		found := false
		for i := range guids {
			if guids[i].Provider == provider {
				// Prefer the critic rating for Rotten Tomatoes
				if guids[i].Rating == "" || siloRating.Source == "rt_critic" {
					guids[i].Rating = rating
				}
				found = true
				break
			}
		}
		if !found {
			guids = append(guids, models.MediaItemGuid{Provider: provider, Rating: rating})
		}
	}

	return guids
}

func fetchSeasonsAndEpisodesForShow(ctx context.Context, msConfig config.Config_MediaServer, itemInfo *models.MediaItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Seasons and Episodes for Show %s", utils.MediaItemInfo(*itemInfo),
	), logging.LevelDebug)
	defer logAction.Complete()

	// Get the file paths for all episodes of the show
	filePaths, Err := getItemFilePaths(ctx, msConfig, itemInfo.RatingKey)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "catalog", "series", itemInfo.RatingKey, "seasons")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// Decode the Response
	var seasonsResp SiloSeasonsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &seasonsResp, "Silo Show Seasons Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	var seasons []models.MediaItemSeason
	for _, siloSeason := range seasonsResp.Items {
		season := models.MediaItemSeason{
			RatingKey:    siloSeason.ContentID,
			SeasonNumber: siloSeason.SeasonNumber,
			Title:        siloSeason.Title,
			Episodes:     []models.MediaItemEpisode{},
		}

		// Construct the URL for the Silo server API request
		u, _ = apiURL(msConfig.URL, "catalog", "series", itemInfo.RatingKey, "seasons", strconv.Itoa(siloSeason.SeasonNumber), "episodes")

		// Make the HTTP Request to Silo
		_, respBody, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}

		// Decode the Response
		var episodesResp SiloEpisodesResponse
		Err = httpx.DecodeResponseToJSON(ctx, respBody, &episodesResp, "Silo Season Episodes Response")
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}

		for _, siloEpisode := range episodesResp.Items {
			// Skip episodes that do not have a file, or are from another season
			if len(siloEpisode.Files) == 0 || siloEpisode.SeasonNumber != siloSeason.SeasonNumber {
				continue
			}

			// Silo has one item for every library it is in, so use the file in this item's library
			// If the library paths are known and none of the files are in the library, the episode is not in this library
			episodeFile := siloEpisode.Files[0]
			if libraryPathsKnown(msConfig, itemInfo.LibraryTitle) {
				foundInLibrary := false
				for _, file := range siloEpisode.Files {
					if isPathInLibrary(msConfig, filePaths[file.FileID], itemInfo.LibraryTitle) {
						episodeFile = file
						foundInLibrary = true
						break
					}
				}
				if !foundInLibrary {
					continue
				}
			}

			season.Episodes = append(season.Episodes, models.MediaItemEpisode{
				RatingKey:     siloEpisode.ContentID,
				Title:         siloEpisode.Title,
				SeasonNumber:  siloEpisode.SeasonNumber,
				EpisodeNumber: siloEpisode.EpisodeNumber,
				File: models.MediaItemFile{
					Path:     filePaths[episodeFile.FileID],
					Size:     episodeFile.FileSize,
					Duration: siloEpisode.Runtime * 60 * 1000,
				},
			})
		}

		if len(season.Episodes) == 0 {
			continue
		}
		seasons = append(seasons, season)
	}

	// Sort the seasons by Season Number
	sort.Slice(seasons, func(i, j int) bool {
		return seasons[i].SeasonNumber < seasons[j].SeasonNumber
	})

	itemInfo.Series = &models.MediaItemSeries{Seasons: seasons}
	return logging.LogErrorInfo{}
}

func getItemFilePaths(ctx context.Context, msConfig config.Config_MediaServer, contentID string) (filePaths map[string]string, Err logging.LogErrorInfo) {
	filePaths = map[string]string{} // Key: Silo File ID

	cursor := ""
	for {
		// Construct the URL for the Silo server API request
		u, err := apiURL(msConfig.URL, "admin", "items", contentID, "files")
		if err != nil {
			message, help, detail := logging.Error_BaseUrlParsing(err)
			return filePaths, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
		}
		query := u.Query()
		query.Set("limit", "200")
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		u.RawQuery = query.Encode()

		// Make the HTTP Request to Silo
		_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
		if Err.Message != "" {
			return filePaths, Err
		}

		// Decode the Response
		var siloResp SiloItemFilesResponse
		Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Item Files Response")
		if Err.Message != "" {
			return filePaths, Err
		}

		for _, file := range siloResp.Items {
			filePaths[file.ID] = file.FilePath
		}

		if !siloResp.Page.HasMore || siloResp.Page.NextCursor == "" {
			break
		}
		cursor = siloResp.Page.NextCursor
	}

	return filePaths, logging.LogErrorInfo{}
}

// libraryStore returns the library cache of the media server
func libraryStore(msConfig config.Config_MediaServer) *cache.MediaServerLibraryCache {
	return cache.MediaServerLibraryStore(msConfig.ID, msConfig.IsMain())
}

func libraryPathsKnown(msConfig config.Config_MediaServer, libraryTitle string) bool {
	section, found := libraryStore(msConfig).GetSectionByTitle(libraryTitle)
	return found && len(section.Paths) > 0
}

func isPathInLibrary(msConfig config.Config_MediaServer, filePath string, libraryTitle string) bool {
	section, found := libraryStore(msConfig).GetSectionByTitle(libraryTitle)
	if !found || filePath == "" {
		return false
	}
	for _, libraryPath := range section.Paths {
		libraryPath = strings.TrimRight(libraryPath, "/")
		if libraryPath != "" && (filePath == libraryPath || strings.HasPrefix(filePath, libraryPath+"/")) {
			return true
		}
	}
	return false
}
