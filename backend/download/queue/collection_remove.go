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

// RemoveCollectionFromQueue deletes every queue file (in-progress, warning, or
// error) belonging to a collection entry. Entries are matched by LibraryTitle +
// RatingKey, mirroring the filename AddCollectionToQueue wrote.
func RemoveCollectionFromQueue(ctx context.Context, deleteItem models.CollectionQueueItem) (deleted int, Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Remove Collection Queue Entry for '%s' [%s]",
			deleteItem.CollectionItem.Title, deleteItem.CollectionItem.RatingKey),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	deleted = 0

	files, readErr := os.ReadDir(CollectionFolderPath)
	if readErr != nil {
		logAction.SetError("Failed to read collection download queue folder",
			"Ensure that the application has permission to read the download queue folder",
			map[string]any{
				"error": readErr.Error(),
				"path":  CollectionFolderPath,
			})
		return deleted, Err
	}

	// QuoteMeta escapes regex metacharacters in the library title / rating key so
	// the pattern matches the literal filename AddCollectionToQueue wrote.
	pattern := fmt.Sprintf(`^(error_|warning_)?%s_%s_\d+\.json$`,
		regexp.QuoteMeta(strings.ReplaceAll(deleteItem.CollectionItem.LibraryTitle, " ", `_`)),
		regexp.QuoteMeta(deleteItem.CollectionItem.RatingKey),
	)
	re := regexp.MustCompile(pattern)

	for _, file := range files {
		if file.IsDir() || !re.MatchString(file.Name()) {
			continue
		}

		filePath := path.Join(CollectionFolderPath, file.Name())
		if delErr := os.Remove(filePath); delErr != nil {
			logAction.SetError("Failed to delete item from collection download queue",
				"Ensure that the application has permission to delete files from the download queue folder",
				map[string]any{
					"error": delErr.Error(),
					"file":  filePath,
				})
			return deleted, Err
		}

		logAction.AppendResult("deleted_file", filePath)
		deleted++
	}

	logAction.AppendResult("total_deleted", deleted)
	return deleted, Err
}
