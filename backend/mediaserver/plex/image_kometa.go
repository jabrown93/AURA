package plex

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// kometaAssetExtension is the on-disk extension AURA writes for Kometa assets.
// MediUX originals are JPEG, and Kometa does not validate content type against the
// extension, so a fixed ".jpg" keeps the folder-per-item layout unambiguous.
const kometaAssetExtension = ".jpg"

// illegalFilesystemChars matches characters that are unsafe in a directory name across
// the platforms AURA supports. Used when deriving an asset folder from a title (collections).
var illegalFilesystemChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// saveImageKometa writes a downloaded media-item image into the Kometa asset directory
// using Kometa's folder-per-item (asset_folders: true) naming conventions:
//
//	<AssetDirectory>/<ASSET_NAME>/poster.jpg
//	<AssetDirectory>/<ASSET_NAME>/background.jpg
//	<AssetDirectory>/<ASSET_NAME>/Season##.jpg   (Season00 for specials)
//	<AssetDirectory>/<ASSET_NAME>/S##E##.jpg      (episode title cards)
//
// where ASSET_NAME is the exact name of the folder the movie file lives in, or the show's
// folder name. It is intentionally non-fatal to the caller: any error is returned so the
// apply flow can log a warning and continue.
func saveImageKometa(ctx context.Context, p *Plex, item *models.MediaItem, imageFile models.ImageFile, imageData []byte) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Saving Kometa Asset %s for %s",
		utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	assetName, Err := kometaAssetName(ctx, p, item)
	if Err.Message != "" {
		return Err
	}

	fileName, ok := kometaFileName(imageFile)
	if !ok {
		logAction.SetError(
			fmt.Sprintf("Unsupported image type '%s' for Kometa asset", imageFile.Type),
			"Only poster, backdrop, season_poster, special_season_poster and titlecard are written as Kometa assets",
			map[string]any{"image_type": imageFile.Type},
		)
		return *logAction.Error
	}

	assetDir := path.Join(config.Current.Images.Kometa.AssetDirectory, assetName)
	logAction.AppendResult("kometa_asset_dir", assetDir)
	logAction.AppendResult("kometa_file_name", fileName)

	return writeKometaAsset(assetDir, fileName, imageData, logAction)
}

// SaveKometaAssetWithName writes downloaded image bytes into the Kometa asset directory using a
// caller-supplied asset folder name, rather than deriving it from the Plex file path. This lets
// callers save Kometa assets for items that can no longer be resolved on the media server, using
// a folder name obtained elsewhere (e.g. from Sonarr/Radarr). It performs no Plex upload.
//
// It returns the on-disk file name written (e.g. "poster.jpg", "Season01.jpg") so callers can
// build a matching Kometa image ID. ok=false means the image type is not written as a Kometa
// asset (the caller should skip it); a non-empty Err means the write itself failed.
func SaveKometaAssetWithName(ctx context.Context, assetName string, imageFile models.ImageFile, imageData []byte) (fileName string, ok bool, Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Saving Kometa Asset '%s' into folder '%s'", imageFile.Type, assetName), logging.LevelDebug)
	defer logAction.Complete()

	fileName, ok = kometaFileName(imageFile)
	if !ok {
		return "", false, logging.LogErrorInfo{}
	}

	assetDir := path.Join(config.Current.Images.Kometa.AssetDirectory, assetName)
	logAction.AppendResult("kometa_asset_dir", assetDir)
	logAction.AppendResult("kometa_file_name", fileName)

	Err = writeKometaAsset(assetDir, fileName, imageData, logAction)
	return fileName, true, Err
}

// kometaAssetName derives the Kometa ASSET_NAME (the media item's folder name) for a
// movie or show. For movies it is the parent directory of the movie file; for shows it is
// the base name of the series location.
func kometaAssetName(ctx context.Context, p *Plex, item *models.MediaItem) (string, logging.LogErrorInfo) {
	switch item.Type {
	case "movie":
		// Fetch full movie details if the file path is not yet populated (mirrors saveImageLocally).
		if item.Movie == nil {
			if _, Err := p.GetMediaItemDetails(ctx, item); Err.Message != "" {
				return "", Err
			}
		}
		if item.Movie == nil || item.Movie.File.Path == "" {
			return "", kometaAssetNameError(item, "movie file path unavailable")
		}
		// Convert first so path.Base sees forward slashes when running against Windows paths.
		filePath := convertWindowsPathToDockerPath(item.Movie.File.Path)
		assetName := path.Base(path.Dir(filePath))
		if assetName == "" || assetName == "." || assetName == "/" {
			return "", kometaAssetNameError(item, "could not determine movie asset folder name")
		}
		return assetName, logging.LogErrorInfo{}
	case "show":
		if item.Series == nil || item.Series.Location == "" {
			return "", kometaAssetNameError(item, "series location unavailable")
		}
		location := convertWindowsPathToDockerPath(item.Series.Location)
		assetName := path.Base(location)
		if assetName == "" || assetName == "." || assetName == "/" {
			return "", kometaAssetNameError(item, "could not determine show asset folder name")
		}
		return assetName, logging.LogErrorInfo{}
	default:
		return "", kometaAssetNameError(item, "unsupported media item type")
	}
}

func kometaAssetNameError(item *models.MediaItem, reason string) logging.LogErrorInfo {
	return logging.LogErrorInfo{
		Message: fmt.Sprintf("Failed to determine Kometa asset folder for %s: %s", item.Title, reason),
		Help:    "Ensure the media item has been fully loaded from Plex and has a valid file path",
		Detail:  map[string]any{"title": item.Title, "type": item.Type, "reason": reason},
	}
}

// kometaFileName returns the Kometa asset file name for a given image type, or ok=false if
// the image type is not written as a Kometa asset.
func kometaFileName(imageFile models.ImageFile) (string, bool) {
	switch imageFile.Type {
	case "poster":
		return "poster" + kometaAssetExtension, true
	case "backdrop":
		return "background" + kometaAssetExtension, true
	case "season_poster":
		if imageFile.SeasonNumber == nil {
			return "", false
		}
		return fmt.Sprintf("Season%02d%s", *imageFile.SeasonNumber, kometaAssetExtension), true
	case "special_season_poster":
		return "Season00" + kometaAssetExtension, true
	case "titlecard":
		if imageFile.SeasonNumber == nil || imageFile.EpisodeNumber == nil {
			return "", false
		}
		return fmt.Sprintf("S%02dE%02d%s", *imageFile.SeasonNumber, *imageFile.EpisodeNumber, kometaAssetExtension), true
	default:
		return "", false
	}
}

// writeKometaAsset creates the asset folder if needed, removes any stale same-base-name
// files with a different extension (so Kometa's glob is unambiguous), and writes the file,
// overwriting an existing asset of the same name.
func writeKometaAsset(assetDir, fileName string, imageData []byte, logAction *logging.LogAction) logging.LogErrorInfo {
	if err := os.MkdirAll(assetDir, os.ModePerm); err != nil {
		logAction.SetError("Failed to create Kometa asset directory", "Ensure the Kometa asset directory is writable",
			map[string]any{"error": err.Error(), "path": assetDir})
		return *logAction.Error
	}

	removeConflictingExtensions(assetDir, fileName)

	savedFilePath := path.Join(assetDir, fileName)
	if err := os.WriteFile(savedFilePath, imageData, 0o644); err != nil {
		logAction.SetError("Failed to write Kometa asset file", "Ensure the Kometa asset directory is writable",
			map[string]any{"error": err.Error(), "path": savedFilePath})
		return *logAction.Error
	}
	logAction.AppendResult("kometa_saved_file_path", savedFilePath)
	return logging.LogErrorInfo{}
}

// removeConflictingExtensions deletes sibling files that share fileName's base name but use
// a different extension (e.g. removes poster.png when writing poster.jpg). Best-effort.
func removeConflictingExtensions(assetDir, fileName string) {
	base := strings.TrimSuffix(fileName, path.Ext(fileName))
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		if ext == kometaAssetExtension {
			continue
		}
		conflict := path.Join(assetDir, base+ext)
		if _, err := os.Stat(conflict); err == nil {
			_ = os.Remove(conflict)
		}
	}
}

// sanitizeForFilesystem turns an arbitrary title into a safe single-segment folder name.
// Used for collections, which have no on-disk folder to borrow a name from.
func sanitizeForFilesystem(name string) string {
	cleaned := illegalFilesystemChars.ReplaceAllString(name, "")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, ".")
	return strings.TrimSpace(cleaned)
}

// SaveCollectionImageKometa writes a collection poster/background into the Kometa asset
// directory as <AssetDirectory>/<Collection Name>/poster.jpg or background.jpg. Non-fatal.
func SaveCollectionImageKometa(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile, imageData []byte) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Saving Kometa Collection Asset for %s", collectionItem.Title,
	), logging.LevelDebug)
	defer logAction.Complete()

	assetName := sanitizeForFilesystem(collectionItem.Title)
	if assetName == "" {
		logAction.SetError("Failed to determine Kometa asset folder for collection",
			"Collection title is empty after sanitization",
			map[string]any{"title": collectionItem.Title})
		return *logAction.Error
	}

	fileName := "poster" + kometaAssetExtension
	if imageFile.Type == "collection_backdrop" || imageFile.Type == "backdrop" {
		fileName = "background" + kometaAssetExtension
	}

	assetDir := path.Join(config.Current.Images.Kometa.AssetDirectory, assetName)
	logAction.AppendResult("kometa_asset_dir", assetDir)
	logAction.AppendResult("kometa_file_name", fileName)

	return writeKometaAsset(assetDir, fileName, imageData, logAction)
}

// UploadImageBytes uploads raw image bytes directly to a Plex item (used by the Kometa
// asset importer, which reads images from disk rather than downloading from MediUX).
// imageType selects the Plex endpoint: "backdrop"/"collection_backdrop" -> /arts, everything
// else -> /posters.
func (p *Plex) UploadImageBytes(ctx context.Context, ratingKey string, imageType string, data []byte) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Uploading %s image bytes to rating key %s", imageType, ratingKey), logging.LevelDebug)
	defer logAction.Complete()

	endpoint := "posters"
	if imageType == "backdrop" || imageType == "collection_backdrop" {
		endpoint = "arts"
	}

	// makeRequest defaults a missing Content-Type to application/json, which is wrong for
	// raw image bytes; label the payload with its detected image MIME type instead.
	headers := AddPlexHeaders(config.Current.MediaServer, map[string]string{
		"Content-Type": http.DetectContentType(data),
	})

	url := fmt.Sprintf("%s/library/metadata/%s/%s", config.Current.MediaServer.URL, ratingKey, endpoint)
	resp, respBody, Err := httpx.MakeHTTPRequest(ctx, url, "POST", headers, 60, data, "Plex")
	if Err.Message != "" {
		return Err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logAction.SetError(fmt.Sprintf("Plex Server returned a %d status code", resp.StatusCode),
			"Check the response from the server for more details",
			map[string]any{"status_code": resp.StatusCode, "error_body": string(respBody)})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

// ResolveRatingKey exposes the internal rating-key resolution for a given media item and
// image file (posters use the item key; season/episode assets resolve to the season/episode
// rating key). Returns "" when the target does not exist on the server.
func (p *Plex) ResolveRatingKey(item models.MediaItem, imageFile models.ImageFile) string {
	return getItemRatingKeyFromImageFile(item, imageFile)
}
