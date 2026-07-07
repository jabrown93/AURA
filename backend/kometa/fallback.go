package kometa

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver/plex"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"context"
	"fmt"
)

// fallbackEnabled reports whether the Sonarr/Radarr → Kometa fallback can run given the current
// config. It requires Kometa mode itself (which owns the asset directory) plus the opt-in toggle,
// and is Plex-only (matching Kometa mode).
func fallbackEnabled() bool {
	k := config.Current.Images.Kometa
	return k.Enabled && k.SonarrRadarrFallback && k.AssetDirectory != "" && config.Current.MediaServer.Type == "Plex"
}

// SaveViaSonarrRadarrFallback writes the selected MediUX images from the given poster sets into the
// Kometa asset directory for an item that could not be resolved on the media server. The asset
// folder name is resolved from Sonarr/Radarr (the same folder Kometa reads), so the assets land in
// the right place even though the media server lost the item. On success it registers a synthetic
// "Kometa" saved set — indistinguishable from a normal Kometa import — so the UI reflects the save
// and the re-apply / auto-download logic never tries to push it back to the still-missing item.
//
// handled=false means the fallback did not apply (disabled, unsupported type, no matching
// Sonarr/Radarr app, or the item is not in Sonarr/Radarr); the caller should keep its normal
// failure behavior. A non-empty Err alongside handled=true is a partial failure (some assets were
// not written); the images that did write are still registered.
func SaveViaSonarrRadarrFallback(ctx context.Context, item models.MediaItem, posterSets []models.DBPosterSetDetail) (handled bool, registered bool, Err logging.LogErrorInfo) {
	if !fallbackEnabled() {
		return false, false, logging.LogErrorInfo{}
	}
	if item.Type != "movie" && item.Type != "show" {
		return false, false, logging.LogErrorInfo{}
	}
	if item.TMDB_ID == "" || item.LibraryTitle == "" {
		return false, false, logging.LogErrorInfo{}
	}

	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Kometa: Sonarr/Radarr fallback for %s", item.Title), logging.LevelInfo)
	defer logAction.Complete()

	folderName, found, srErr := sonarr_radarr.GetItemFolderName(ctx, item.TMDB_ID, item.Type, item.LibraryTitle)
	if !found {
		// Item is not in a matching Sonarr/Radarr instance (or none is configured for this
		// library); there is nothing to fall back to — let the caller use its normal error path.
		if srErr.Message != "" {
			logAction.AppendWarning("sonarr_radarr_lookup", map[string]any{"error": srErr.Message})
		}
		return false, false, logging.LogErrorInfo{}
	}
	logAction.AppendResult("kometa_asset_folder", folderName)

	var written []models.ImageFile
	var selected models.SelectedTypes
	var writeErrors []string

	for _, set := range posterSets {
		for _, image := range set.Images {
			if !imageSelected(set.SelectedTypes, image) {
				continue
			}

			// Download the raw bytes from MediUX (this path never touches the media server).
			formatDate := image.Modified.Format("20060102150405")
			data, _, dErr := mediux.GetImage(ctx, image.ID, formatDate, mediux.ImageQualityOriginal)
			if dErr.Message != "" {
				writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", image.Type, dErr.Message))
				continue
			}

			fileName, ok, wErr := plex.SaveKometaAssetWithName(ctx, folderName, image, data)
			if !ok {
				// Image type is not a Kometa asset type; skip silently.
				continue
			}
			if wErr.Message != "" {
				writeErrors = append(writeErrors, fmt.Sprintf("%s: %s", image.Type, wErr.Message))
				continue
			}

			// Record the asset as a Kometa image so it is served from disk like a normal import.
			written = append(written, models.ImageFile{
				ID:            imageIDForAsset(folderName, fileName),
				Type:          image.Type,
				Modified:      image.Modified,
				ItemTMDB_ID:   item.TMDB_ID,
				SeasonNumber:  image.SeasonNumber,
				EpisodeNumber: image.EpisodeNumber,
			})
			markSelected(&selected, image.Type)
		}
	}

	logAction.AppendResult("images_written", len(written))

	if len(writeErrors) > 0 {
		Err = logging.LogErrorInfo{
			Message: fmt.Sprintf("%d Kometa asset(s) could not be saved", len(writeErrors)),
			Help:    "Check the Kometa asset directory permissions and MediUX availability",
			Detail:  map[string]any{"errors": writeErrors},
		}
	}

	if len(written) == 0 {
		// The item matched Sonarr/Radarr (so it is "handled" and the caller should not fall through
		// to its own error), but nothing was written.
		return true, false, Err
	}

	registered = registerImportedItem(ctx, &item, folderName, written, selected)
	logAction.AppendResult("registered_in_db", registered)
	return true, registered, Err
}

// SaveSavedSetsViaSonarrRadarrFallback loads the item's saved auto-download poster sets from the
// database and runs the Sonarr/Radarr → Kometa fallback for them. Used by flows where the item is
// already persisted (auto-download check, Plex event listener) and the media server can no longer
// resolve it. handled=false means either the fallback did not apply or the item has no saved
// auto-download sets to write.
func SaveSavedSetsViaSonarrRadarrFallback(ctx context.Context, item models.MediaItem) (handled bool, registered bool, Err logging.LogErrorInfo) {
	if !fallbackEnabled() {
		return false, false, logging.LogErrorInfo{}
	}

	savedItems, dbErr := database.GetAllSavedSets(ctx, models.DBFilter{
		ItemTMDB_ID:      item.TMDB_ID,
		ItemLibraryTitle: item.LibraryTitle,
	})
	if dbErr.Message != "" {
		return false, false, dbErr
	}

	var sets []models.DBPosterSetDetail
	for _, savedItem := range savedItems.Items {
		for _, posterSet := range savedItem.PosterSets {
			// Match the re-apply / auto-download contract: only auto-download sets are re-applied.
			if posterSet.AutoDownload {
				sets = append(sets, posterSet)
			}
		}
	}
	if len(sets) == 0 {
		return false, false, logging.LogErrorInfo{}
	}

	return SaveViaSonarrRadarrFallback(ctx, item, sets)
}

// imageSelected reports whether an image should be written given the poster set's selected types,
// mirroring the download-queue selection logic (season 0 posters are treated as specials).
func imageSelected(selected models.SelectedTypes, image models.ImageFile) bool {
	switch image.Type {
	case "poster":
		return selected.Poster
	case "backdrop":
		return selected.Backdrop
	case "season_poster":
		if image.SeasonNumber != nil && *image.SeasonNumber == 0 {
			return selected.SpecialSeasonPoster
		}
		return selected.SeasonPoster
	case "special_season_poster":
		return selected.SpecialSeasonPoster
	case "titlecard":
		return selected.Titlecard
	}
	return false
}
