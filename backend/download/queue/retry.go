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

// RetryFromQueue re-queues an errored download entry by stripping the "error_"
// prefix from its file, turning it back into an in-progress entry that the
// processor picks up on its next run. The rename is atomic, so there is no
// window where the entry is lost (unlike a remove-then-add sequence).
func RetryFromQueue(ctx context.Context, retryItem models.DBSavedItem) (retried int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Retry Entry for %s",
			utils.MediaItemInfo(retryItem.MediaItem)),
		logging.LevelDebug)

	Err = logging.LogErrorInfo{}
	retried = 0

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
		return retried, Err
	}

	// Match only errored files for this item (LibraryTitle + TMDB ID). This uses
	// the same name construction as RemoveFromQueue so retry and delete target
	// the same set of files. QuoteMeta escapes regex metacharacters (e.g. a
	// library named "Movies (4K)") so the pattern matches the literal filename
	// AddToQueue wrote rather than being interpreted as regex syntax.
	pattern := fmt.Sprintf(`^error_%s_%s_\d+\.json$`,
		regexp.QuoteMeta(strings.ReplaceAll(retryItem.MediaItem.LibraryTitle, " ", `_`)),
		regexp.QuoteMeta(retryItem.MediaItem.TMDB_ID),
	)
	re := regexp.MustCompile(pattern)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !re.MatchString(file.Name()) {
			continue
		}

		// Strip the "error_" prefix so the processor treats it as in-progress again.
		oldPath := path.Join(FolderPath, file.Name())
		newPath := path.Join(FolderPath, strings.TrimPrefix(file.Name(), "error_"))

		if renameErr := os.Rename(oldPath, newPath); renameErr != nil {
			logAction.SetError("Failed to re-queue errored item",
				"Ensure that the application has permission to modify files in the download queue folder",
				map[string]any{
					"error": renameErr.Error(),
					"from":  oldPath,
					"to":    newPath,
				})
			logAction.Complete()
			return retried, Err
		}

		logAction.AppendResult("requeued_file", newPath)
		retried++
	}

	logAction.AppendResult("total_retried", retried)
	logAction.Complete()
	return retried, Err
}
