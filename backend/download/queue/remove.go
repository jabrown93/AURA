package downloadqueue

import (
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

func RemoveFromQueue(ctx context.Context, deleteItem models.DBSavedItem) (deleted int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Remove Entry for %s",
			utils.MediaItemInfo(deleteItem.MediaItem)),
		logging.LevelDebug)

	Err = logging.LogErrorInfo{}
	deleted = 0

	// Read all files in the download queue folder
	files, readErr := os.ReadDir(FolderPath)
	if readErr != nil {
		logAction.SetError("Failed to read download queue folder",
			"Ensure that the application has permission to read the download queue folder",
			map[string]any{
				"error": readErr.Error(),
				"path":  FolderPath,
			})
		logAction.Complete()
		return deleted, Err
	}

	// Build a regex pattern to match the file name for the item to be deleted.
	// QuoteMeta escapes any regex metacharacters in the library title (e.g.
	// "Movies (4K)") so the pattern matches the literal filename AddToQueue wrote.
	pattern := fmt.Sprintf(`^(error_|warning_)?%s_%s_\d+\.json$`,
		regexp.QuoteMeta(strings.ReplaceAll(deleteItem.MediaItem.LibraryTitle, " ", `_`)),
		regexp.QuoteMeta(deleteItem.MediaItem.TMDB_ID),
	)
	re := regexp.MustCompile(pattern)

	// Loop through each file and check if it matches the item to be deleted
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if re.MatchString(file.Name()) {
			// If a matching file is found, delete it
			filePath := path.Join(FolderPath, file.Name())
			delErr := os.Remove(filePath)
			if delErr != nil {
				logAction.SetError("Failed to delete item from download queue",
					"Ensure that the application has permission to delete files from the download queue folder",
					map[string]any{
						"error": delErr.Error(),
						"file":  filePath,
					})
				logAction.Complete()
				return deleted, Err
			}

			logAction.AppendResult("deleted_file", filePath)
			deleted++
		}
	}

	logAction.AppendResult("total_deleted", deleted)
	logAction.Complete()
	return deleted, Err
}
