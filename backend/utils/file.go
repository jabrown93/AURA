package utils

import (
	"aura/logging"
	"context"
	"fmt"
	"os"
	"path"
)

func CreateFolderIfNotExists(ctx context.Context, folderPath string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Checking if folder exists", logging.LevelTrace)
	defer logAction.Complete()

	_, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		// Folder does not exist, create it
		err := os.MkdirAll(folderPath, 0755)
		if err != nil {
			logAction.SetError("Failed to create folder", fmt.Sprintf("Ensure the path %s is accessible and writable.", folderPath), map[string]any{
				"error":       err.Error(),
				"folder_path": folderPath,
			})
			return *logAction.Error
		}
	}
	return logging.LogErrorInfo{}
}

// GetExtensionFromContentType maps an image Content-Type/MIME type to a file
// extension (including the leading dot). Defaults to ".jpg" for unknown or
// empty content types, since JPEG is the most common MediUX image format.
func GetExtensionFromContentType(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func ClearAllFilesFromFolder(ctx context.Context, folderPath string) (clearCount int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Clearing files from folder", logging.LevelInfo)
	defer logAction.Complete()

	clearCount = 0
	Err = logging.LogErrorInfo{}

	files, err := os.ReadDir(folderPath)
	if err != nil {
		logAction.SetError("Failed to read folder", "Ensure the folder exists and is accessible", map[string]any{
			"path":  folderPath,
			"error": err.Error()})
		return 0, *logAction.Error
	}

	if len(files) == 0 {
		return clearCount, Err
	}

	for _, file := range files {
		if file.IsDir() {
			subFolderCount, subErr := ClearAllFilesFromFolder(ctx, path.Join(folderPath, file.Name()))
			if subErr.Message != "" {
				logAction.AppendWarning(file.Name(), map[string]any{
					"error": subErr.Message,
				})
				continue
			}
			clearCount += subFolderCount
		} else {
			err := os.Remove(path.Join(folderPath, file.Name()))
			if err != nil {
				logAction.AppendWarning(file.Name(), map[string]any{
					"error": err.Error(),
				})
				continue
			}
			clearCount++
		}
	}

	return clearCount, Err
}
