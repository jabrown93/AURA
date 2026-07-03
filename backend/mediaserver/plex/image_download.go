package plex

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

func (p *Plex) DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Downloading and Applying %s Image for %s", utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item)),
		logging.LevelDebug)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	// Determine the Item Rating Key from Plex
	itemRatingKey := getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		logAction.SetError("Failed to determine Rating Key for Media Item", "Ensure the Media Item and Image File data are correct", nil)
		return *logAction.Error
	}
	logAction.AppendResult("image_rating_key", itemRatingKey)

	localEnabled := config.Current.Images.SaveImagesLocally.Enabled
	kometaEnabled := config.Current.Images.Kometa.Enabled

	// If neither local save nor Kometa mode is enabled, skip downloading the image bytes
	// and apply it directly to Plex via the MediUX URL.
	if !localEnabled && !kometaEnabled {
		return applyImageToMediaItemViaMediuxURL(ctx, item, itemRatingKey, imageFile)
	}

	// Get the Image from MediUX
	// mediux.GetImage will handle checking the temp folder and caching based on config
	formatDate := imageFile.Modified.Format("20060102150405")
	imageData, _, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	// Save the Image Locally (Plex Local Media Assets naming, next to content)
	if localEnabled {
		_, Err = saveImageLocally(ctx, p, item, imageFile, imageData)
		if Err.Message != "" {
			return Err
		}
	}

	// Save the Image into the Kometa asset directory (Kometa naming conventions).
	// This is non-fatal: a failure to write the asset must never block applying the image to Plex.
	if kometaEnabled {
		if kometaErr := saveImageKometa(ctx, p, item, imageFile, imageData); kometaErr.Message != "" {
			logAction.AppendWarning("kometa_save_failed", map[string]any{
				"error": kometaErr.Message,
			})
		}
	}

	// Apply the image to Plex via the MediUX URL (preserves existing immediate-apply behavior).
	Err = applyImageToMediaItemViaMediuxURL(ctx, item, itemRatingKey, imageFile)
	if Err.Message != "" {
		return Err
	}
	return Err
	// }
	// else {
	// 	// Refresh the Plex item
	// 	RefreshItemMetadata(ctx, item, itemRatingKey, false)

	// 	// Get the Plex Poster Key
	// 	failedToGetPosterKey := false
	// 	imageKey, Err := findNewImage(ctx, item, itemRatingKey, imageFile.Type, currentImages)
	// 	if Err.Message != "" {
	// 		failedToGetPosterKey = true
	// 		logAction.AppendWarning("message", "Failed to find new image in Plex after local save")
	// 		logAction.AppendWarning("warning", map[string]any{
	// 			"error": Err,
	// 		})
	// 	} else {
	// 		logAction.AppendResult("new_image_key", imageKey)
	// 	}

	// 	// If failedOnGetPosters is true, use the MediUX URL to set the poster
	// 	if failedToGetPosterKey {
	// 		applyImageToMediaItemViaMediuxURL(ctx, item, itemRatingKey, imageFile)
	// 		if Err.Message != "" {
	// 			return Err
	// 		}
	// 		return Err
	// 	}

	// 	// Set the Poster using the Plex Image Key
	// 	Err = applyImageToMediaItem(ctx, item, itemRatingKey, imageKey, imageFile.Type)
	// 	if Err.Message != "" {
	// 		return Err
	// 	}
	// }

	// return Err
}

func saveImageLocally(ctx context.Context, p *Plex, item *models.MediaItem, imageFile models.ImageFile, imageData []byte) (isCustomLocalPath bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Saving %s Image for %s",
		utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	isCustomLocalPath = false
	Err = logging.LogErrorInfo{}

	newFilePath := ""
	newFileName := ""

	getFilePathAction := logAction.AddSubAction(fmt.Sprintf("Determining File Path for %s (%s)", imageFile.Type, item.Title), logging.LevelTrace)
	switch item.Type {
	case "movie":
		// Handle Movie Specific Logic
		// If the item.Movie is nil, get the full movie details from the media server
		if item.Movie == nil {
			_, Err = p.GetMediaItemDetails(ctx, item)
			if Err.Message != "" {
				return isCustomLocalPath, Err
			}
			getFilePathAction.AppendResult("fetched_movie_from_server", true)
		}

		newFilePath = path.Dir(item.Movie.File.Path)
		getFilePathAction.AppendResult("movie_file_path", newFilePath)
		switch imageFile.Type {
		case "poster":
			newFileName = "poster.jpg"
		case "backdrop":
			newFileName = "backdrop.jpg"
		}
	case "show":
		// Handle show-specific logic
		newFilePath = item.Series.Location
		getFilePathAction.AppendResult("series_location", newFilePath)
		switch imageFile.Type {
		case "poster":
			newFileName = "poster.jpg"
		case "backdrop":
			newFileName = "backdrop.jpg"
		case "season_poster":
			seasonNumber := utils.FormatIntAsTwoDigitString(*imageFile.SeasonNumber)
			newFileName = fmt.Sprintf("season%s-poster.jpg", seasonNumber)
		case "special_season_poster":
			newFileName = "season-specials-poster.jpg"
		case "titlecard":
			episodeNamingConvention := config.Current.Images.SaveImagesLocally.EpisodeNamingConvention
			// For titlecards, get the file path from Plex
			episodePath := getEpisodePathFromImageFile(*item, imageFile)
			getFilePathAction.AppendResult("episode_path_lookup", episodePath)
			if episodePath != "" {
				newFilePath = path.Dir(episodePath)
				switch episodeNamingConvention {
				case "match":
					newFileName = path.Base(episodePath)
					newFileName = newFileName[:len(newFileName)-len(path.Ext(newFileName))] + "-thumb.jpg"
				case "static":
					filename := path.Base(episodePath)
					re := regexp.MustCompile(`(?i)\bS?\d{1,3}[Ex]\d{1,3}\b`)
					trimmed := strings.TrimSuffix(filename, path.Ext(filename))
					matchedString := re.FindString(trimmed)
					if matchedString != "" {
						newFileName = matchedString + ".jpg"
					} else {
						// If we failed to get the season and episode numbers, try and get them from the file struct
						newFileName = fmt.Sprintf("S%02dE%02d.jpg", imageFile.SeasonNumber, imageFile.EpisodeNumber)
					}
				default:
					getFilePathAction.SetError("Invalid Episode Naming Convention",
						"EpisodeNamingConvention must be either 'match' or 'static'",
						map[string]any{
							"EpisodeNamingConvention": episodeNamingConvention,
						})
					return isCustomLocalPath, *getFilePathAction.Error
				}
			} else {
				getFilePathAction.SetError("Failed to determine file path for titlecard",
					"Could not find episode path in Plex data",
					map[string]any{
						"rating_key": item.RatingKey,
					})
				return isCustomLocalPath, *getFilePathAction.Error
			}
		}
	default:
		getFilePathAction.SetError("Unsupported Media Item Type for Poster Update",
			"Only 'movie' and 'show' types are supported for poster updates",
			map[string]any{
				"item_type": item.Type,
			})
		return isCustomLocalPath, *getFilePathAction.Error
	}
	getFilePathAction.AppendResult("newFilePath", newFilePath)
	getFilePathAction.Complete()

	if config.Current.Images.SaveImagesLocally.Enabled && config.Current.Images.SaveImagesLocally.Path != "" {
		isCustomLocalPath = true
		newPathAction := logAction.AddSubAction("Building New File Path for Local Image Save", logging.LevelDebug)
		// Build newFilePath based on library, content, and config path
		libraryRoot := ""
		libSection, exists := cache.LibraryStore.GetSectionByTitle(item.LibraryTitle)
		newPathAction.AppendResult("library_title", item.LibraryTitle)
		if exists && len(libSection.Paths) > 0 {
			newPathAction.AppendResult("library_found_in_cache", true)
			// Library exists in cache (e.g. /data/media/movies or /data/media/shows)
			if len(libSection.Paths) > 1 {
				newPathAction.AppendResult("multiple_library_paths_found", true)
				// If there are multiple library paths, we need to determine which one is correct based on the item path
				itemPath := ""
				if item.Movie != nil {
					itemPath = item.Movie.File.Path
				} else if item.Series != nil {
					itemPath = item.Series.Location
				}
				newPathAction.AppendResult("item_path_for_library_determination", itemPath)

				for _, libPath := range libSection.Paths {
					if strings.HasPrefix(itemPath, libPath) {
						libraryRoot = libPath
						newPathAction.AppendResult("matched_library_path", libPath)
						break
					}
				}
				if libraryRoot == "" {
					newPathAction.SetError("Failed to match library path for media item",
						"Multiple library paths found but none matched the media item's file path",
						map[string]any{
							"item_path":     itemPath,
							"library_paths": libSection.Paths,
						})
					return isCustomLocalPath, *newPathAction.Error
				}
			} else {
				libraryRoot = libSection.Paths[0]
			}
			newPathAction.AppendResult("library_root", libraryRoot)

			// Get last part of library root (e.g. "movies" or "shows")
			libraryPath := path.Base(libraryRoot)
			newPathAction.AppendResult("library_path", libraryPath)

			// Get path before library name (e.g. /data/media/)
			remainingLibraryPath := strings.TrimSuffix(libraryRoot, libraryPath)
			newPathAction.AppendResult("remaining_library_path", remainingLibraryPath)

			// Get relative path from newFilePath (e.g. movies/Inception (2020), shows/Breaking Bad/Season 01)
			relativePath := strings.TrimPrefix(newFilePath, remainingLibraryPath)
			relativePath = strings.TrimLeft(relativePath, string(os.PathSeparator))
			newPathAction.AppendResult("relative_path", relativePath)

			// Final path: /local/images/movies/Inception (2020), etc.
			newFilePath = path.Join(config.Current.Images.SaveImagesLocally.Path, relativePath)
			newPathAction.AppendResult("final_path", newFilePath)
		} else {
			newPathAction.AppendResult("library_found_in_cache", false)
			// Fallback: build path from Plex info
			libraryPath := ""
			contentPath := ""
			seasonPath := ""

			if imageFile.Type != "titlecard" {
				// For movies or posters/backdrops
				contentPath = path.Base(newFilePath)
				newPathAction.AppendResult("content_path", contentPath)

				libraryPath = path.Base(path.Dir(newFilePath))
				newPathAction.AppendResult("library_path", libraryPath)

				// Final path:  /local/images/movies/Inception (2020)
				//				/local/images/shows/Breaking Bad
				newFilePath = path.Join(config.Current.Images.SaveImagesLocally.Path, libraryPath, contentPath)
				newPathAction.AppendResult("final_path", newFilePath)
			} else if item.Type == "show" && (imageFile.Type == "titlecard") {
				// For shows with season_posters/titlecard
				seasonPath = path.Base(newFilePath)
				newPathAction.AppendResult("season_path", seasonPath)

				contentPath = path.Base(path.Dir(newFilePath))
				newPathAction.AppendResult("content_path", contentPath)

				libraryPath = path.Base(path.Dir(path.Dir(newFilePath)))
				newPathAction.AppendResult("library_path", libraryPath)

				// Final path: /local/images/shows/Breaking Bad/Season 01
				newFilePath = path.Join(config.Current.Images.SaveImagesLocally.Path, libraryPath, contentPath, seasonPath)
				newPathAction.AppendResult("final_path", newFilePath)
			} else {
				// Error: unable to determine path
				newPathAction.SetError("Failed to determine library path", "Ensure the library exists in Plex and has a valid path",
					map[string]any{
						"title": item.Title,
						"type":  item.Type,
						"file":  imageFile.Type,
					})
				return isCustomLocalPath, *newPathAction.Error
			}
		}
		newPathAction.Complete()
	}

	createFileAction := logAction.AddSubAction("Saving Image to New File Path", logging.LevelDebug)
	savedFilePath := path.Join(newFilePath, newFileName)
	newFilePath = convertWindowsPathToDockerPath(newFilePath)
	savedFilePath = convertWindowsPathToDockerPath(savedFilePath)

	// Ensure the directory exists
	err := os.MkdirAll(newFilePath, os.ModePerm)
	if err != nil {
		createFileAction.SetError("Failed to create directory", "Ensure the directory can be created",
			map[string]any{
				"error": err.Error(),
				"path":  newFilePath,
			})
		return isCustomLocalPath, *createFileAction.Error
	}

	// Create the new file
	newFile, err := os.Create(savedFilePath)
	if err != nil {
		createFileAction.SetError("Failed to create file", "Ensure the file can be created",
			map[string]any{
				"error": err.Error(),
				"path":  savedFilePath,
			})
		return isCustomLocalPath, *createFileAction.Error
	}
	defer newFile.Close()

	// Write the image data to the new file
	_, err = newFile.Write(imageData)
	if err != nil {
		createFileAction.SetError("Failed to write image data to file", "Ensure the file is writable",
			map[string]any{
				"error": err.Error(),
				"path":  savedFilePath,
			})
		return isCustomLocalPath, *createFileAction.Error
	}
	createFileAction.AppendResult("saved_file_path", savedFilePath)
	createFileAction.Complete()

	makeFileBytesUnique(savedFilePath)

	return isCustomLocalPath, Err
}

func convertWindowsPathToDockerPath(windowsPath string) string {
	if !config.Current.Images.SaveImagesLocally.RunningOnWindows {
		return windowsPath
	} else {
		logging.LOGGER.Debug().Timestamp().Msg("ConvertWindowsPathToDockerPath called, fixing path for Windows")
	}
	// Replace backslashes with forward slashes
	dockerPath := strings.ReplaceAll(windowsPath, "\\", "/")

	// Handle drive letter conversion (e.g., C:/ to /C/)
	if len(dockerPath) > 1 && dockerPath[1] == ':' {
		driveLetter := string(dockerPath[0])
		dockerPath = "/" + driveLetter + dockerPath[2:]
	}

	return dockerPath
}

func makeFileBytesUnique(filePath string) error {
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	r := make([]byte, 6)
	if _, err := rand.Read(r); err != nil {
		return err
	}

	tag := fmt.Sprintf("\nAURA:%s:%s",
		time.Now().UTC().Format("20060102T150405.000000000Z"),
		hex.EncodeToString(r),
	)

	if _, err := f.Write([]byte(tag)); err != nil {
		return err
	}

	now := time.Now()
	return os.Chtimes(filePath, now, now)
}
