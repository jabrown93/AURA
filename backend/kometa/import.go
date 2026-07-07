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

// configuredSubfolders returns the distinct, sanitized per-library subfolders (relative to the
// asset directory) from the current config, so the import scan can descend into each one.
func configuredSubfolders() []string {
	seen := make(map[string]bool)
	subs := make([]string, 0, len(config.Current.Images.Kometa.LibraryAssetFolders))
	for _, raw := range config.Current.Images.Kometa.LibraryAssetFolders {
		sub := config.SanitizeKometaSubfolder(raw)
		if sub == "" || seen[sub] {
			continue
		}
		seen[sub] = true
		subs = append(subs, sub)
	}
	return subs
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

	folders, err := Scan(assetDir, configuredSubfolders())
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
			Folder:  folder.RelDir,
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
// in the database. The precedence guard runs BEFORE any upload: image types already owned
// by a non-Kometa (MediUX) set — and items the user has ignored — are never pushed to Plex,
// so an import cannot silently replace an AURA-managed image on the server.
func processItem(ctx context.Context, plexClient *plex.Plex, assetDir string, folder ScannedFolder, item *models.MediaItem, result *ImportResult) {
	outcome := FolderOutcome{Folder: folder.RelDir, Outcome: "matched", Detail: item.LibraryTitle}

	// Precedence guard: load AURA's existing records first so owned types are skipped
	// before their bytes ever reach Plex.
	ignored, _, existingSets, logErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle)
	if logErr.Message != "" {
		// Without the existing sets the guard cannot run; skip the item entirely rather
		// than risk overwriting a MediUX-managed image on Plex.
		outcome.Outcome = "error"
		outcome.Detail = "failed to load existing AURA records; skipped to protect MediUX selections"
		result.Folders = append(result.Folders, outcome)
		return
	}
	if ignored {
		outcome.Outcome = "skipped"
		outcome.Detail = "item is ignored in AURA"
		result.Folders = append(result.Folders, outcome)
		return
	}
	owned := ownedTypes(existingSets)

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
		if typeOwned(owned, asset.Type) {
			outcome.ImagesSkippedOwned++
			result.ImagesSkippedOwned++
			continue
		}

		imageFile := models.ImageFile{
			ID:            imageIDForAsset(folder.RelDir, asset.FileName),
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

		data, err := os.ReadFile(filepath.Join(assetDir, filepath.FromSlash(folder.RelDir), asset.FileName))
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

	outcome.ManagedByAura = outcome.ImagesSkippedOwned > 0
	if outcome.ManagedByAura {
		result.SkippedManagedByAura++
	}

	if len(uploadedImages) == 0 {
		if outcome.ManagedByAura && outcome.ImagesFailed == 0 {
			outcome.Detail = "all image types are managed by AURA; nothing uploaded"
		} else {
			outcome.Detail = "no images uploaded"
		}
		result.Folders = append(result.Folders, outcome)
		return
	}

	outcome.RegisteredInDB = registerImportedItem(ctx, item, folder.Name, uploadedImages, selected)
	if outcome.RegisteredInDB {
		result.ItemsRegistered++
	}
	result.Folders = append(result.Folders, outcome)
}

// ownedTypes merges the selected image types of every non-Kometa (MediUX) set for an item.
// These types are AURA-managed: the import must neither upload them to Plex nor claim them
// in the database.
func ownedTypes(existingSets []models.DBSavedSet) models.SelectedTypes {
	var owned models.SelectedTypes
	for _, s := range existingSets {
		if IsKometaSetID(s.ID) {
			continue
		}
		owned.Poster = owned.Poster || s.SelectedTypes.Poster
		owned.Backdrop = owned.Backdrop || s.SelectedTypes.Backdrop
		owned.SeasonPoster = owned.SeasonPoster || s.SelectedTypes.SeasonPoster
		owned.SpecialSeasonPoster = owned.SpecialSeasonPoster || s.SelectedTypes.SpecialSeasonPoster
		owned.Titlecard = owned.Titlecard || s.SelectedTypes.Titlecard
	}
	return owned
}

// typeOwned reports whether an asset's image type is claimed in the given selected types.
func typeOwned(owned models.SelectedTypes, assetType string) bool {
	switch assetType {
	case "poster":
		return owned.Poster
	case "backdrop":
		return owned.Backdrop
	case "season_poster":
		return owned.SeasonPoster
	case "special_season_poster":
		return owned.SpecialSeasonPoster
	case "titlecard":
		return owned.Titlecard
	}
	return false
}

// registerImportedItem writes a synthetic "Kometa Import" poster set for the item. The
// caller (processItem) has already excluded AURA-owned types and ignored items, so
// `selected` only contains types this import may claim.
func registerImportedItem(ctx context.Context, item *models.MediaItem, folderName string, images []models.ImageFile, selected models.SelectedTypes) (registered bool) {
	if !anySelected(selected) {
		return false
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
		return false
	}
	return true
}

// processCollection uploads a folder's poster/background to a matched collection. Collection
// assets are applied to Plex only (not recorded as AURA saved sets).
func processCollection(ctx context.Context, plexClient *plex.Plex, assetDir string, folder ScannedFolder, coll *models.CollectionItem, result *ImportResult) {
	outcome := FolderOutcome{Folder: folder.RelDir, Outcome: "collection", Detail: coll.LibraryTitle}

	for _, asset := range folder.Assets {
		// Collections only carry a poster and a background.
		if asset.Type != "poster" && asset.Type != "backdrop" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(assetDir, filepath.FromSlash(folder.RelDir), asset.FileName))
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
