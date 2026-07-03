package kometa

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediaserver/plex"
	"aura/models"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StartImport kicks off a Kometa asset import in the background. It returns false if an
// import is already running or if Kometa mode is not enabled/configured for Plex.
func StartImport() (started bool) {
	if !importEnabled() {
		return false
	}
	if !tryStart() {
		return false
	}
	go runImport()
	return true
}

// importEnabled reports whether Kometa import can run given the current config.
func importEnabled() bool {
	k := config.Current.Images.Kometa
	return k.Enabled && k.AssetDirectory != "" && config.Current.MediaServer.Type == "Plex"
}

// runImport performs the import synchronously against a background context and records the
// result. It recovers from panics so a bad asset directory can never crash the process.
func runImport() {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Kometa Asset Import")
	logAction := ld.AddAction("Import Kometa Assets", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer ld.Log()

	result := &ImportResult{StartedAt: time.Now()}
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Sprintf("panic during Kometa import: %v", r)
			logging.LOGGER.Error().Timestamp().Interface("panic", r).Msg("Kometa import panicked")
		}
		result.FinishedAt = time.Now()
		finish(result)
	}()

	assetDir := config.Current.Images.Kometa.AssetDirectory

	// Refresh the library/collection cache so matching runs against current data.
	mediaserver.GetAllLibrarySectionsAndItems(ctx, true)

	folders, err := Scan(assetDir)
	if err != nil {
		result.Error = err.Error()
		logAction.SetError("Failed to scan Kometa asset directory", "Ensure the asset directory is readable", map[string]any{"error": err.Error(), "path": assetDir})
		return
	}
	result.FoldersScanned = len(folders)

	plexClient := &plex.Plex{Config: config.Current.MediaServer}

	for _, folder := range folders {
		if len(folder.Assets) == 0 {
			// No recognized assets in this folder; nothing to import.
			continue
		}

		if items := matchFolderToItems(folder.Name); len(items) > 0 {
			result.Matched++
			for _, item := range items {
				processItem(ctx, plexClient, assetDir, folder, item, result)
			}
			continue
		}

		if coll, ok := matchFolderToCollection(folder.Name); ok {
			result.Collections++
			processCollection(ctx, plexClient, assetDir, folder, coll, result)
			continue
		}

		result.UnmatchedFolders++
		result.Folders = append(result.Folders, FolderOutcome{
			Folder:  folder.Name,
			Outcome: "unmatched",
			Detail:  "no matching media item or collection in the library",
		})
	}

	logAction.AppendResult("folders_scanned", result.FoldersScanned)
	logAction.AppendResult("images_uploaded", result.ImagesUploaded)
	logAction.AppendResult("items_registered", result.ItemsRegistered)
	logAction.AppendResult("unmatched_folders", result.UnmatchedFolders)
}

// processItem uploads a folder's assets to a single matched media item and registers them
// in the database (subject to the precedence guard).
func processItem(ctx context.Context, plexClient *plex.Plex, assetDir string, folder ScannedFolder, item *models.MediaItem, result *ImportResult) {
	outcome := FolderOutcome{Folder: folder.Name, Outcome: "matched", Detail: item.LibraryTitle}

	// Load full details so season/episode rating keys resolve (and to verify existence).
	found, Err := mediaserver.GetMediaItemDetails(ctx, item)
	if Err.Message != "" || !found {
		outcome.Outcome = "error"
		outcome.Detail = "failed to load media item details from Plex"
		result.Folders = append(result.Folders, outcome)
		return
	}

	var uploadedImages []models.ImageFile
	var selected models.SelectedTypes

	for _, asset := range folder.Assets {
		imageFile := models.ImageFile{
			ID:            imageIDForAsset(folder.Name, asset.FileName),
			Type:          asset.Type,
			Modified:      asset.ModTime,
			ItemTMDB_ID:   item.TMDB_ID,
			SeasonNumber:  asset.Season,
			EpisodeNumber: asset.Episode,
		}

		ratingKey := plexClient.ResolveRatingKey(*item, imageFile)
		if ratingKey == "" {
			// Season/episode not present on the server; skip this asset.
			outcome.ImagesFailed++
			result.ImagesFailed++
			continue
		}

		data, err := os.ReadFile(filepath.Join(assetDir, folder.Name, asset.FileName))
		if err != nil {
			outcome.ImagesFailed++
			result.ImagesFailed++
			continue
		}

		if uErr := plexClient.UploadImageBytes(ctx, ratingKey, asset.Type, data); uErr.Message != "" {
			outcome.ImagesFailed++
			result.ImagesFailed++
			continue
		}

		outcome.ImagesUploaded++
		result.ImagesUploaded++
		uploadedImages = append(uploadedImages, imageFile)
		markSelected(&selected, asset.Type)
	}

	if len(uploadedImages) == 0 {
		outcome.Detail = "no images uploaded"
		result.Folders = append(result.Folders, outcome)
		return
	}

	registered, managed := registerImportedItem(ctx, item, folder.Name, uploadedImages, selected)
	outcome.RegisteredInDB = registered
	outcome.ManagedByAura = managed
	if registered {
		result.ItemsRegistered++
	}
	if managed {
		result.SkippedManagedByAura++
	}
	result.Folders = append(result.Folders, outcome)
}

// registerImportedItem writes a synthetic "Kometa Import" poster set for the item. The
// precedence guard clears any image type already owned by a non-Kometa (MediUX) set so an
// import never overwrites the user's AURA selections; if nothing remains, registration is
// skipped and the item is reported as managed by AURA.
func registerImportedItem(ctx context.Context, item *models.MediaItem, folderName string, images []models.ImageFile, selected models.SelectedTypes) (registered, managed bool) {
	_, _, existingSets, _ := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle)
	for _, s := range existingSets {
		if IsKometaSetID(s.ID) {
			continue
		}
		if s.SelectedTypes.Poster {
			selected.Poster = false
		}
		if s.SelectedTypes.Backdrop {
			selected.Backdrop = false
		}
		if s.SelectedTypes.SeasonPoster {
			selected.SeasonPoster = false
		}
		if s.SelectedTypes.SpecialSeasonPoster {
			selected.SpecialSeasonPoster = false
		}
		if s.SelectedTypes.Titlecard {
			selected.Titlecard = false
		}
	}

	if !anySelected(selected) {
		// Every uploaded type is owned by a MediUX set; leave AURA's records untouched.
		return false, true
	}

	now := time.Now()
	dbItem := models.DBSavedItem{
		MediaItem: *item,
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:          setIDForItem(item.TMDB_ID, item.LibraryTitle),
					Title:       "Kometa Import: " + folderName,
					Type:        item.Type,
					UserCreated: "Kometa",
					DateCreated: now,
					DateUpdated: now,
				},
				Images: images,
			},
			SelectedTypes:  selected,
			AutoDownload:   false,
			LastDownloaded: now,
		}},
	}

	if Err := database.UpsertSavedItem(ctx, dbItem); Err.Message != "" {
		return false, false
	}
	return true, false
}

// processCollection uploads a folder's poster/background to a matched collection. Collection
// assets are applied to Plex only (not recorded as AURA saved sets).
func processCollection(ctx context.Context, plexClient *plex.Plex, assetDir string, folder ScannedFolder, coll *models.CollectionItem, result *ImportResult) {
	outcome := FolderOutcome{Folder: folder.Name, Outcome: "collection", Detail: coll.LibraryTitle}

	for _, asset := range folder.Assets {
		// Collections only carry a poster and a background.
		if asset.Type != "poster" && asset.Type != "backdrop" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(assetDir, folder.Name, asset.FileName))
		if err != nil {
			outcome.ImagesFailed++
			result.ImagesFailed++
			continue
		}
		if uErr := plexClient.UploadImageBytes(ctx, coll.RatingKey, asset.Type, data); uErr.Message != "" {
			outcome.ImagesFailed++
			result.ImagesFailed++
			continue
		}
		outcome.ImagesUploaded++
		result.ImagesUploaded++
	}

	result.Folders = append(result.Folders, outcome)
}

func markSelected(selected *models.SelectedTypes, assetType string) {
	switch assetType {
	case "poster":
		selected.Poster = true
	case "backdrop":
		selected.Backdrop = true
	case "season_poster":
		selected.SeasonPoster = true
	case "special_season_poster":
		selected.SpecialSeasonPoster = true
	case "titlecard":
		selected.Titlecard = true
	}
}

func anySelected(s models.SelectedTypes) bool {
	return s.Poster || s.Backdrop || s.SeasonPoster || s.SpecialSeasonPoster || s.Titlecard
}
