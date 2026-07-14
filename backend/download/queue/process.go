package downloadqueue

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/kometa"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"
)

// showHasAnyEpisode reports whether the series has at least one episode across all of
// its seasons on the media server. Force-preload only pre-stages assets for shows that
// exist with at least one episode.
func showHasAnyEpisode(series *models.MediaItemSeries) bool {
	if series == nil {
		return false
	}
	for _, season := range series.Seasons {
		if len(season.Episodes) > 0 {
			return true
		}
	}
	return false
}

// finalizeQueueFile moves a processed queue entry to its terminal state. On a
// clean success the file is removed. When there are errors/warnings the entry is
// enriched with the collected reasons and a failed_at timestamp, written back in
// place, then atomically renamed to the error_/warning_ prefix so the API and UI
// can surface why it failed. Files that could not be parsed into a usable entry
// (empty TMDB ID) keep their original bytes and are simply renamed, so we never
// fabricate a broken entry.
func finalizeQueueFile(filePath, fileName string, item models.DBSavedItem, fileErrors, fileWarnings []string) error {
	hasErrors := len(fileErrors) > 0
	hasWarnings := len(fileWarnings) > 0

	if !hasErrors && !hasWarnings {
		return os.Remove(filePath)
	}

	prefix := "warning_"
	if hasErrors {
		prefix = "error_"
	}
	destPath := path.Join(FolderPath, prefix+fileName)

	// Only enrich entries we actually parsed. Unparseable files have no usable
	// identity, so move the original bytes untouched.
	if item.MediaItem.TMDB_ID == "" {
		return os.Rename(filePath, destPath)
	}

	// Record why the entry failed so the UI can display it.
	item.QueueErrors = fileErrors
	item.QueueWarnings = fileWarnings
	now := time.Now()
	item.FailedAt = &now

	enriched, marshalErr := json.Marshal(item)
	if marshalErr != nil {
		// Fall back to a bare rename so the entry still leaves the in-progress state.
		return os.Rename(filePath, destPath)
	}

	// Write the enriched entry to a temp file first. GetQueueItems and
	// ProcessQueueItems only look at ".json" files, so the ".tmp" file is
	// invisible to them while the original in-progress ".json" stays intact and
	// fully readable. We then atomically rename the temp into the final
	// error_/warning_ name and drop the original, so a concurrent reader never
	// observes a half-written ".json" (which would fail to decode and transiently
	// drop the entry). The temp name is deterministic, so a crash leaves at most
	// one stale ".tmp" that the next run overwrites.
	tmpPath := filePath + ".tmp"
	if writeErr := os.WriteFile(tmpPath, enriched, 0644); writeErr != nil {
		return os.Rename(filePath, destPath)
	}
	if renameErr := os.Rename(tmpPath, destPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return os.Rename(filePath, destPath)
	}
	// The enriched entry now exists under its final name; drop the original.
	return os.Remove(filePath)
}

func ProcessQueueItems() {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue Processing")
	logAction := ld.AddAction("Processing Download Queue", logging.LevelInfo)
	defer logAction.Complete()
	ctx = logging.WithCurrentAction(ctx, logAction)

	// Read all JSON files in the download-queue directory
	files, err := os.ReadDir(FolderPath)
	if err != nil {
		logging.LOGGER.Warn().Timestamp().Err(err).Msg("Failed to read download queue directory")
		logAction.SetError("Failed to read download queue directory", "Ensure the directory exists and is accessible",
			map[string]any{
				"error":      err.Error(),
				"folderPath": FolderPath,
			})
		return
	}

	if len(files) == 0 {
		logAction.AppendResult("result", "queue is empty")
		return
	}

	// Process each file in the directory
	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".json" {
			continue
		}

		// If a file starts with "error_" or "warning_", skip it
		if len(file.Name()) > 6 && (file.Name()[:6] == "error_" || file.Name()[:8] == "warning_") {
			continue
		}

		ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue - Processing")
		subAction := ld.AddAction(fmt.Sprintf("Processing file: %s", file.Name()), logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, subAction)

		// Reset the Latest Info for this file
		LatestInfo.Status = LAST_STATUS_PROCESSING
		LatestInfo.Message = fmt.Sprintf("Processing file: %s", file.Name())
		LatestInfo.Errors = []string{}
		LatestInfo.Warnings = []string{}

		// Create an array of errors and warnings for this file
		fileErrors := []string{}
		fileWarnings := []string{}

		filePath := path.Join(FolderPath, file.Name())

		// Declared up front so the finalizeAndNotify closure can enrich it with
		// the collected errors/warnings when moving the file to its terminal state.
		var queueItem models.DBSavedItem

		finalizeAndNotify := func(
			mediaItem models.MediaItem,
			set models.DBPosterSetDetail,
			tmdbPoster string,
			tmdbBackdrop string,
		) {
			issues := FileIssues{Errors: fileErrors, Warnings: fileWarnings}
			// Record the terminal status before notifying: SendNotification skips
			// the LatestInfo update when notifications are disabled (the default),
			// which would otherwise leave the banner stuck on "Processing...".
			setLatestInfoTerminal(mediaItem.Title, fileErrors, fileWarnings)
			SendNotification(issues, mediaItem, set, tmdbPoster, tmdbBackdrop)

			if err := finalizeQueueFile(filePath, file.Name(), queueItem, fileErrors, fileWarnings); err != nil {
				subAction.AppendWarning(fmt.Sprintf("file_%s", file.Name()), "Failed to move or delete processed file")
			}
			ld.Log()
		}

		// Read and parse the JSON file
		data, err := os.ReadFile(filePath)
		if err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("read file failed: %v", err))
			finalizeAndNotify(models.MediaItem{}, models.DBPosterSetDetail{}, "", "")
			continue
		}

		if err := json.Unmarshal(data, &queueItem); err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("parse json failed: %v", err))
			finalizeAndNotify(models.MediaItem{}, models.DBPosterSetDetail{}, "", "")
			continue
		}

		if queueItem.MediaItem.RatingKey == "" || queueItem.MediaItem.Title == "" || queueItem.MediaItem.LibraryTitle == "" || queueItem.MediaItem.TMDB_ID == "" {
			fileErrors = append(fileErrors, "media item missing required fields: ratingKey/title/libraryTitle/tmdbId")
			finalizeAndNotify(queueItem.MediaItem, models.DBPosterSetDetail{}, "", "")
			continue
		}

		if len(queueItem.PosterSets) == 0 {
			fileWarnings = append(fileWarnings, "no poster sets found")
			finalizeAndNotify(queueItem.MediaItem, models.DBPosterSetDetail{}, "", "")
			continue
		}

		mediuxItemInfo, mErr := mediux.GetBaseItemInfoByTMDB_ID(queueItem.MediaItem.TMDB_ID, queueItem.MediaItem.Type)
		if mErr.Message != "" {
			fileWarnings = append(fileWarnings, fmt.Sprintf("mediux lookup failed: %s", mErr.Message))
		}

		found, mediaErr := mediaserver.GetMediaItemDetails(ctx, &queueItem.MediaItem)

		// The queue file snapshotted this item's rating key when it was enqueued. Media-server
		// rating keys are not stable (Plex reassigns them when an item is removed and re-added,
		// e.g. a file move / re-import), so a key that was valid at enqueue time can be dead by the
		// time the queue drains — the fetch above then 404s. Before failing, re-resolve the current
		// key from the in-memory library cache by the stable TMDB ID and retry the fetch once.
		//
		// This runs only after a genuine 404, never for an item whose queued key still resolves, so
		// a valid entry is never redirected to a different item that merely shares the TMDB ID
		// (duplicate editions in the same library). If the cache holds no newer key, the queued key
		// is left as-is and the existing Sonarr/Radarr fallback + error path below handles it.
		if mediaserver.IsItemNotFound(mediaErr) {
			if cached, ok := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(queueItem.MediaItem.LibraryTitle, queueItem.MediaItem.TMDB_ID); ok && cached.RatingKey != "" && cached.RatingKey != queueItem.MediaItem.RatingKey {
				subAction.AppendResult(fmt.Sprintf("rating_key_refresh_%s", file.Name()), fmt.Sprintf("%s -> %s (stale key re-resolved by TMDB %s)", queueItem.MediaItem.RatingKey, cached.RatingKey, queueItem.MediaItem.TMDB_ID))
				queueItem.MediaItem.RatingKey = cached.RatingKey
				found, mediaErr = mediaserver.GetMediaItemDetails(ctx, &queueItem.MediaItem)
			}
		}

		if mediaErr.Message != "" || !found {
			// The media server can't resolve the item (e.g. Plex returns a 404 for a stale rating
			// key). If the Sonarr/Radarr → Kometa fallback is enabled and the item exists in
			// Sonarr/Radarr, still write the downloaded images into the Kometa asset folder (folder
			// name from the Sonarr/Radarr path) and record a synthetic Kometa set, so the
			// assets are not lost just because the media server lost the item.
			//
			// Only fall back on a genuine 404 (a stale/removed rating key). Transient, auth, and
			// server errors are left to fail so the download is retried and eventually applied.
			if mediaserver.IsItemNotFound(mediaErr) {
				if handled, _, kErr := kometa.SaveViaSonarrRadarrFallback(ctx, queueItem.MediaItem, queueItem.PosterSets); handled {
					if kErr.Message != "" {
						fileWarnings = append(fileWarnings, fmt.Sprintf("kometa fallback: %s", kErr.Message))
					}
					// Treat as a completed download: the images were saved to the Kometa asset folder
					// even though the media-server apply was skipped.
					finalizeAndNotify(
						queueItem.MediaItem,
						models.DBPosterSetDetail{},
						mediuxItemInfo.TMDB_PosterPath,
						mediuxItemInfo.TMDB_BackdropPath,
					)
					continue
				}
			}

			fileErrors = append(fileErrors, fmt.Sprintf("media server lookup failed for '%s' in '%s': %s", queueItem.MediaItem.Title, queueItem.MediaItem.LibraryTitle, mediaErr.Message))
			// Stop retry flood: mark file as error_ immediately
			finalizeAndNotify(
				queueItem.MediaItem,
				models.DBPosterSetDetail{},
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)
			continue
		}

		for _, posterSet := range queueItem.PosterSets {
			setErrors := []string{}
			setWarnings := []string{}

			if posterSet.ID == "" || posterSet.Type == "" || posterSet.Title == "" {
				setErrors = append(setErrors, "poster set missing required fields: id/type/title")
				fileErrors = append(fileErrors, setErrors...)
				SendNotification(
					FileIssues{Errors: setErrors, Warnings: setWarnings},
					queueItem.MediaItem,
					posterSet,
					mediuxItemInfo.TMDB_PosterPath,
					mediuxItemInfo.TMDB_BackdropPath,
				)
				continue
			}

			if !posterSet.SelectedTypes.Poster &&
				!posterSet.SelectedTypes.Backdrop &&
				!posterSet.SelectedTypes.SeasonPoster &&
				!posterSet.SelectedTypes.SpecialSeasonPoster &&
				!posterSet.SelectedTypes.Titlecard {
				setWarnings = append(setWarnings, "poster set has no selected image types")
				fileWarnings = append(fileWarnings, setWarnings...)
				SendNotification(
					FileIssues{Errors: setErrors, Warnings: setWarnings},
					queueItem.MediaItem,
					posterSet,
					mediuxItemInfo.TMDB_PosterPath,
					mediuxItemInfo.TMDB_BackdropPath,
				)
				continue
			}

			LatestInfo.Message = fmt.Sprintf("%s (Set: %s)", queueItem.MediaItem.Title, posterSet.ID)

			// When the set is flagged to force-preload missing seasons/episodes, Kometa asset
			// mode is enabled, and the show exists on the server with at least one episode, we
			// pre-stage the assets for seasons/episodes not yet on the server instead of skipping.
			preloadEligible := posterSet.ForcePreloadMissing &&
				config.Current.Images.Kometa.Enabled &&
				showHasAnyEpisode(queueItem.MediaItem.Series)

			for idx, image := range posterSet.Images {
				// preloadThisImage routes an image to a Kometa-asset-only write (no server apply)
				// because its target season/episode is missing from the media server.
				preloadThisImage := false
				switch image.Type {
				case "poster":
					if !posterSet.SelectedTypes.Poster {
						continue
					}
				case "backdrop":
					if !posterSet.SelectedTypes.Backdrop {
						continue
					}
				case "season_poster":
					if image.SeasonNumber == nil {
						continue
					}
					// Gate on the selected image type first so it applies to both present and preloaded seasons
					if *image.SeasonNumber == 0 {
						if !posterSet.SelectedTypes.SpecialSeasonPoster {
							continue
						}
					} else {
						if !posterSet.SelectedTypes.SeasonPoster {
							continue
						}
					}
					// Check if the Media Item contains the season number for this image
					mediaItemHasSeason := false
					if queueItem.MediaItem.Series != nil {
						for _, season := range queueItem.MediaItem.Series.Seasons {
							if *image.SeasonNumber == season.SeasonNumber {
								mediaItemHasSeason = true
								break
							}
						}
					}
					if !mediaItemHasSeason {
						// Season missing on the server: preload it as a Kometa asset if eligible, else skip
						if !preloadEligible {
							continue
						}
						preloadThisImage = true
					}
				case "titlecard":
					if !posterSet.SelectedTypes.Titlecard {
						continue
					}
					// Check if the Media Item contains the Season and Episode numbers for this image
					mediaItemHasEpisode := false
					if queueItem.MediaItem.Series != nil {
						for _, season := range queueItem.MediaItem.Series.Seasons {
							for _, episode := range season.Episodes {
								if image.SeasonNumber != nil && *image.SeasonNumber != season.SeasonNumber {
									continue
								}
								if image.EpisodeNumber != nil && *image.EpisodeNumber != episode.EpisodeNumber {
									continue
								}
								mediaItemHasEpisode = true
								break
							}
							if mediaItemHasEpisode {
								break
							}
						}
					}
					if !mediaItemHasEpisode {
						// Episode missing on the server: preload it as a Kometa asset if eligible, else skip.
						// A titlecard needs both season and episode numbers to build the Kometa file name.
						if !preloadEligible || image.SeasonNumber == nil || image.EpisodeNumber == nil {
							continue
						}
						preloadThisImage = true
					}
				default:
					subAction.AppendWarning(fmt.Sprintf("file_%s_image_%d", file.Name(), idx), fmt.Sprintf("Image has unrecognized type '%s'", image.Type))
					fileWarnings = append(fileWarnings, fmt.Sprintf("Image '%s' has unrecognized type '%s'", image.Src, image.Type))
					continue
				}

				downloadFileName := utils.GetFileDownloadName(queueItem.MediaItem.Title, image)
				var Err logging.LogErrorInfo
				if preloadThisImage {
					Err = mediaserver.SaveImageAsKometaAssetOnly(ctx, &queueItem.MediaItem, image)
				} else {
					Err = mediaserver.DownloadApplyImageToMediaItem(ctx, &queueItem.MediaItem, image)
				}
				if Err.Message != "" {
					setErrors = append(setErrors, fmt.Sprintf("%s: %s", downloadFileName, Err.Message))
				}
			}

			// Per-set notification (success/warning/error)
			SendNotification(
				FileIssues{Errors: setErrors, Warnings: setWarnings},
				queueItem.MediaItem,
				posterSet,
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)

			fileErrors = append(fileErrors, setErrors...)
			fileWarnings = append(fileWarnings, setWarnings...)
		}

		Err := database.UpsertSavedItem(ctx, queueItem)
		if Err.Message != "" {
			fileErrors = append(fileErrors, fmt.Sprintf("db upsert failed: %s", Err.Message))
			finalizeAndNotify(
				queueItem.MediaItem,
				models.DBPosterSetDetail{},
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)
			continue
		}

		// Success path: per-set SendNotification calls only touched LatestInfo when
		// notifications are enabled, so record the terminal status here too.
		setLatestInfoTerminal(queueItem.MediaItem.Title, fileErrors, fileWarnings)

		if err := finalizeQueueFile(filePath, file.Name(), queueItem, fileErrors, fileWarnings); err != nil {
			fileWarnings = append(fileWarnings, fmt.Sprintf("finalize file failed: %v", err))
		}

		// Handle any labels and tags asynchronously
		go func() {
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue - Labels and Tags Handling")
			logAction := ld.AddAction("Handle Labels and Tags for Added Item", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, logAction)
			defer ld.Log()
			selectedTypes := models.SelectedTypes{}
			for _, posterSet := range queueItem.PosterSets {
				selectedTypes.Poster = selectedTypes.Poster || posterSet.SelectedTypes.Poster
				selectedTypes.Backdrop = selectedTypes.Backdrop || posterSet.SelectedTypes.Backdrop
				selectedTypes.SeasonPoster = selectedTypes.SeasonPoster || posterSet.SelectedTypes.SeasonPoster
				selectedTypes.SpecialSeasonPoster = selectedTypes.SpecialSeasonPoster || posterSet.SelectedTypes.SpecialSeasonPoster
				selectedTypes.Titlecard = selectedTypes.Titlecard || posterSet.SelectedTypes.Titlecard
			}

			mediaserver.AddLabelToMediaItem(ctx, queueItem.MediaItem, selectedTypes)
			sonarr_radarr.HandleTags(ctx, queueItem.MediaItem, selectedTypes)
		}()

		ld.Log()
	}
}
