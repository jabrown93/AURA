package downloadqueue

import (
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
)

// finalizeCollectionQueueFile mirrors finalizeQueueFile for the collection
// queue: on a clean success the file is removed, otherwise the entry is enriched
// with the collected errors/warnings + a failed_at timestamp and atomically
// renamed to the error_/warning_ prefix. Entries we could not parse (empty
// RatingKey) keep their original bytes and are simply renamed.
func finalizeCollectionQueueFile(filePath, fileName string, item models.CollectionQueueItem, fileErrors, fileWarnings []string) error {
	hasErrors := len(fileErrors) > 0
	hasWarnings := len(fileWarnings) > 0

	if !hasErrors && !hasWarnings {
		return os.Remove(filePath)
	}

	prefix := "warning_"
	if hasErrors {
		prefix = "error_"
	}
	destPath := path.Join(CollectionFolderPath, prefix+fileName)

	// Only enrich entries we actually parsed. Unparseable files have no usable
	// identity, so move the original bytes untouched.
	if item.CollectionItem.RatingKey == "" {
		return os.Rename(filePath, destPath)
	}

	item.QueueErrors = fileErrors
	item.QueueWarnings = fileWarnings
	now := time.Now()
	item.FailedAt = &now

	enriched, marshalErr := json.Marshal(item)
	if marshalErr != nil {
		return os.Rename(filePath, destPath)
	}

	// Write to a temp file first, then atomically rename it into the final
	// error_/warning_ name so a concurrent reader never observes a half-written
	// ".json" (see finalizeQueueFile for the full rationale).
	tmpPath := filePath + ".tmp"
	if writeErr := os.WriteFile(tmpPath, enriched, 0644); writeErr != nil {
		return os.Rename(filePath, destPath)
	}
	if renameErr := os.Rename(tmpPath, destPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return os.Rename(filePath, destPath)
	}
	return os.Remove(filePath)
}

// ProcessCollectionQueueItems processes queued collection-image downloads. It is
// the collection counterpart to ProcessQueueItems and is invoked on the same
// cron tick. Unlike the media-item processor it does no media-server item
// lookup, DB upsert, or label/tag handling: a Collection is applied directly by
// its RatingKey via mediaserver.ApplyCollectionImage (which also writes the
// Kometa asset when Kometa mode is enabled).
func ProcessCollectionQueueItems() {
	_, ld := logging.CreateLoggingContext(context.Background(), "Collection Download Queue Processing")
	logAction := ld.AddAction("Processing Collection Download Queue", logging.LevelInfo)
	defer logAction.Complete()

	files, err := os.ReadDir(CollectionFolderPath)
	if err != nil {
		logging.LOGGER.Warn().Timestamp().Err(err).Msg("Failed to read collection download queue directory")
		logAction.SetError("Failed to read collection download queue directory", "Ensure the directory exists and is accessible",
			map[string]any{
				"error":      err.Error(),
				"folderPath": CollectionFolderPath,
			})
		return
	}

	if len(files) == 0 {
		logAction.AppendResult("result", "collection queue is empty")
		return
	}

	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".json" {
			continue
		}

		// Skip already-terminal entries.
		if strings.HasPrefix(file.Name(), "error_") || strings.HasPrefix(file.Name(), "warning_") {
			continue
		}

		ctx, ld := logging.CreateLoggingContext(context.Background(), "Collection Download Queue - Processing")
		subAction := ld.AddAction(fmt.Sprintf("Processing file: %s", file.Name()), logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, subAction)

		LatestInfo.Status = LAST_STATUS_PROCESSING
		LatestInfo.Message = fmt.Sprintf("Processing collection file: %s", file.Name())
		LatestInfo.Errors = []string{}
		LatestInfo.Warnings = []string{}

		fileErrors := []string{}
		fileWarnings := []string{}

		filePath := path.Join(CollectionFolderPath, file.Name())

		var queueItem models.CollectionQueueItem

		finalizeAndNotify := func() {
			// Record the terminal status before notifying (see setLatestInfoTerminal):
			// SendNotification skips the LatestInfo update when notifications are
			// disabled, which would otherwise leave the banner stuck on "Processing...".
			label := queueItem.CollectionItem.Title
			if label == "" {
				label = file.Name()
			}
			setLatestInfoTerminal(fmt.Sprintf("Collection: %s", label), fileErrors, fileWarnings)

			sendCollectionNotification(FileIssues{Errors: fileErrors, Warnings: fileWarnings}, queueItem)
			if err := finalizeCollectionQueueFile(filePath, file.Name(), queueItem, fileErrors, fileWarnings); err != nil {
				subAction.AppendWarning(fmt.Sprintf("file_%s", file.Name()), "Failed to move or delete processed file")
			}
			ld.Log()
		}

		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("read file failed: %v", readErr))
			finalizeAndNotify()
			continue
		}

		if err := json.Unmarshal(data, &queueItem); err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("parse json failed: %v", err))
			finalizeAndNotify()
			continue
		}

		if queueItem.CollectionItem.RatingKey == "" || queueItem.CollectionItem.Title == "" || queueItem.CollectionItem.LibraryTitle == "" {
			fileErrors = append(fileErrors, "collection item missing required fields: ratingKey/title/libraryTitle")
			finalizeAndNotify()
			continue
		}

		if len(queueItem.Images) == 0 {
			fileWarnings = append(fileWarnings, "no images to download")
			finalizeAndNotify()
			continue
		}

		LatestInfo.Message = fmt.Sprintf("Collection: %s", queueItem.CollectionItem.Title)

		collectionItem := queueItem.CollectionItem
		for _, image := range queueItem.Images {
			if image.ID == "" || image.Type == "" {
				fileWarnings = append(fileWarnings, "skipped image with missing id/type")
				continue
			}
			if image.Type != "collection_poster" && image.Type != "collection_backdrop" {
				fileWarnings = append(fileWarnings, fmt.Sprintf("image '%s' has unsupported type '%s'", image.ID, image.Type))
				continue
			}

			Err := mediaserver.ApplyCollectionImage(ctx, &collectionItem, image)
			if Err.Message != "" {
				fileErrors = append(fileErrors, fmt.Sprintf("%s: %s", image.Type, Err.Message))
			}
		}

		finalizeAndNotify()
	}
}

// sendCollectionNotification reuses the shared download-queue notification (and
// its LatestInfo status bookkeeping) by adapting a CollectionQueueItem into the
// MediaItem/DBPosterSetDetail shape SendNotification expects. This deliberately
// avoids adding a new notification template type (a 7-file change) — a queued
// collection reuses the existing "Download Queue" template.
func sendCollectionNotification(issues FileIssues, item models.CollectionQueueItem) {
	mediaItem := models.MediaItem{
		Title:        item.CollectionItem.Title,
		LibraryTitle: item.CollectionItem.LibraryTitle,
		Type:         "collection",
		TMDB_ID:      item.CollectionItem.TMDB_ID,
		RatingKey:    item.CollectionItem.RatingKey,
	}

	posterSet := models.DBPosterSetDetail{
		PosterSet: models.PosterSet{
			BaseSetInfo: models.BaseSetInfo{
				ID:    item.CollectionItem.RatingKey,
				Title: item.CollectionItem.Title,
				Type:  "collection",
			},
			Images: item.Images,
		},
		SelectedTypes: models.SelectedTypes{
			Poster:   collectionImagesInclude(item.Images, "collection_poster"),
			Backdrop: collectionImagesInclude(item.Images, "collection_backdrop"),
		},
	}

	SendNotification(issues, mediaItem, posterSet, "", "")
}

func collectionImagesInclude(images []models.ImageFile, imageType string) bool {
	for _, image := range images {
		if image.Type == imageType {
			return true
		}
	}
	return false
}
