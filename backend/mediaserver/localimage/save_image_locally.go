package localimage

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

// SaveImageLocally saves the image next to the content, or to SaveImagesLocally.Path if it is set
// getMediaItemDetails is the media server's GetMediaItemDetails, used when the item does not have its file info
func SaveImageLocally(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile, imageData []byte, imageType string,
	getMediaItemDetails func(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo)) (Err logging.LogErrorInfo) {
	serverType := config.Current.MediaServer.Type
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Saving %s Image for %s",
		serverType, utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// The image extension must match the actual downloaded content type
	// (MediUX images may be JPEG, PNG, WEBP, or GIF), not be assumed as JPEG
	ext := utils.GetExtensionFromContentType(imageType)

	newFilePath := ""
	newFileName := ""

	getFilePathAction := logAction.AddSubAction(fmt.Sprintf("Determining File Path for %s (%s)", imageFile.Type, item.Title), logging.LevelTrace)

	// If the item does not have its file info, get the full details from the media server
	if (item.Type == "movie" && item.Movie == nil) || (item.Type == "show" && item.Series == nil) {
		_, Err = getMediaItemDetails(ctx, item)
		if Err.Message != "" {
			return Err
		}
		getFilePathAction.AppendResult("fetched_item_from_server", true)
	}
	if item.Type == "movie" && (item.Movie == nil || item.Movie.File.Path == "") {
		getFilePathAction.SetError("Failed to determine file path for movie", fmt.Sprintf("%s did not return a file for this movie", serverType),
			map[string]any{
				"rating_key": item.RatingKey,
			})
		return *getFilePathAction.Error
	}

	// The folder the image is saved in, its name comes from its name template
	episodePath := ""
	switch item.Type {
	case "movie":
		newFilePath = path.Dir(item.Movie.File.Path)
		getFilePathAction.AppendResult("movie_file_path", newFilePath)
	case "show":
		newFilePath = item.Series.Location
		getFilePathAction.AppendResult("series_location", newFilePath)
		// Title cards are saved next to their episode
		if imageFile.Type == "titlecard" {
			episodePath = getEpisodePathFromImageFile(*item, imageFile)
			getFilePathAction.AppendResult("episode_path_lookup", episodePath)
			if episodePath == "" {
				getFilePathAction.SetError("Failed to determine file path for titlecard",
					fmt.Sprintf("Could not find episode path in %s data", serverType),
					map[string]any{
						"rating_key": item.RatingKey,
					})
				return *getFilePathAction.Error
			}
			newFilePath = path.Dir(episodePath)
		}
	default:
		getFilePathAction.SetError("Unsupported Media Item Type for Poster Update",
			"Only 'movie' and 'show' types are supported for poster updates",
			map[string]any{
				"item_type": item.Type,
			})
		return *getFilePathAction.Error
	}
	getFilePathAction.AppendResult("newFilePath", newFilePath)
	getFilePathAction.Complete()

	if config.Current.Images.SaveImagesLocally.Enabled && config.Current.Images.SaveImagesLocally.Path != "" {
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
					return *newPathAction.Error
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
			// Fallback: build path from the media server info
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
				newPathAction.SetError("Failed to determine library path", fmt.Sprintf("Ensure the library exists in %s and has a valid path", serverType),
					map[string]any{
						"title": item.Title,
						"type":  item.Type,
						"file":  imageFile.Type,
					})
				return *newPathAction.Error
			}
		}
		newPathAction.Complete()
	}

	// The name template names the file, and can add folders
	libraryID := ""
	if section, ok := cache.LibraryStore.GetSectionByTitle(item.LibraryTitle); ok {
		libraryID = section.ID
	}
	vars := nameTemplateVars(*item, imageFile, libraryID, episodePath)
	template, standard := nameTemplate(imageFile.Type)
	templated := templatedPath(template, newFilePath, config.Current.Images.SaveImagesLocally.Path, vars, ext)
	if templated == "" {
		templated = templatedPath(standard, newFilePath, "", vars, ext)
	}
	if templated == "" {
		logAction.SetError("Failed to name the image file", "Check the file names in Settings > Images", map[string]any{"image_type": imageFile.Type})
		return *logAction.Error
	}
	newFilePath, newFileName = path.Dir(templated), path.Base(templated)
	logAction.AppendResult("name_template", template)

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
		return *createFileAction.Error
	}

	// Create the new file
	newFile, err := os.Create(savedFilePath)
	if err != nil {
		createFileAction.SetError("Failed to create file", "Ensure the file can be created",
			map[string]any{
				"error": err.Error(),
				"path":  savedFilePath,
			})
		return *createFileAction.Error
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
		return *createFileAction.Error
	}
	createFileAction.AppendResult("saved_file_path", savedFilePath)
	createFileAction.Complete()

	makeFileBytesUnique(savedFilePath)

	return Err
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

func getEpisodePathFromImageFile(item models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber
	episodeNumberFromSet := imageFile.EpisodeNumber
	if seasonNumberFromSet == nil || episodeNumberFromSet == nil || item.Series == nil {
		return ""
	}
	for _, season := range item.Series.Seasons {
		if season.SeasonNumber == *seasonNumberFromSet {
			for _, episode := range season.Episodes {
				if episode.EpisodeNumber == *episodeNumberFromSet && episode.SeasonNumber == *seasonNumberFromSet {
					return episode.File.Path
				}
			}
		}
	}
	return ""
}
