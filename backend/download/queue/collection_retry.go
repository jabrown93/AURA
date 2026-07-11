package downloadqueue

import (
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

// RetryCollectionFromQueue re-queues an errored collection entry by stripping
// the "error_" prefix from its file, turning it back into an in-progress entry
// the processor picks up on its next run. The rename is atomic, so there is no
// window where the entry is lost.
func RetryCollectionFromQueue(ctx context.Context, retryItem models.CollectionQueueItem) (retried int, Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Retry Collection Queue Entry for '%s' [%s]",
			retryItem.CollectionItem.Title, retryItem.CollectionItem.RatingKey),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	retried = 0

	files, readErr := os.ReadDir(CollectionFolderPath)
	if readErr != nil {
		logAction.SetError("Failed to read collection download queue folder",
			"Ensure that the application has permission to read the download queue folder",
			map[string]any{
				"error": readErr.Error(),
				"path":  CollectionFolderPath,
			})
		return retried, Err
	}

	// Match only errored files for this collection (LibraryTitle + RatingKey),
	// using the same name construction as RemoveCollectionFromQueue.
	pattern := fmt.Sprintf(`^error_%s_%s_\d+\.json$`,
		regexp.QuoteMeta(strings.ReplaceAll(retryItem.CollectionItem.LibraryTitle, " ", `_`)),
		regexp.QuoteMeta(retryItem.CollectionItem.RatingKey),
	)
	re := regexp.MustCompile(pattern)

	for _, file := range files {
		if file.IsDir() || !re.MatchString(file.Name()) {
			continue
		}

		oldPath := path.Join(CollectionFolderPath, file.Name())
		newPath := path.Join(CollectionFolderPath, strings.TrimPrefix(file.Name(), "error_"))

		if renameErr := os.Rename(oldPath, newPath); renameErr != nil {
			logAction.SetError("Failed to re-queue errored collection item",
				"Ensure that the application has permission to modify files in the download queue folder",
				map[string]any{
					"error": renameErr.Error(),
					"from":  oldPath,
					"to":    newPath,
				})
			return retried, Err
		}

		logAction.AppendResult("requeued_file", newPath)
		retried++
	}

	logAction.AppendResult("total_retried", retried)
	return retried, Err
}
