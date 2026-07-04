package routes_images

import (
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// GetKometaImage godoc
// @Summary      Get Kometa Asset Image
// @Description  Serve a locally-imported Kometa asset by its image ID (kometa|<folder>/<file>) from the configured asset directory.
// @Tags         Images
// @Produce      image/jpeg,image/png,image/webp
// @Param        asset_id   query     string  true  "Kometa image ID (kometa|<folder>/<file>)"
// @Success      200  {string}  string "Image data"
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/kometa/item [get]
func GetKometaImage(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Kometa Image", logging.LevelInfo)

	assetID := r.URL.Query().Get("asset_id")
	rel := kometa.KometaImageRelPath(assetID)
	if rel == "" {
		logAction.SetError("Invalid Kometa asset ID", "asset_id must be a Kometa image ID (kometa|<folder>/<file>)", map[string]any{"asset_id": assetID})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Kometa image IDs always encode exactly "<folder>/<file>"; reject anything deeper,
	// shallower, or containing relative segments before touching the filesystem.
	folder, file, ok := strings.Cut(rel, "/")
	if !ok || folder == "" || file == "" || strings.Contains(file, "/") ||
		folder == "." || folder == ".." || file == "." || file == ".." ||
		strings.ContainsRune(folder, os.PathSeparator) || strings.ContainsRune(file, os.PathSeparator) {
		logAction.SetError("Invalid Kometa asset path", "asset_id must encode exactly <folder>/<file>", map[string]any{"asset_id": assetID})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Only serve known image types; this endpoint must not expose arbitrary files.
	switch strings.ToLower(filepath.Ext(file)) {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		logAction.SetError("Invalid Kometa asset extension", "Only jpg, jpeg, png and webp assets are served", map[string]any{"asset_id": assetID})
		httpx.SendResponse(w, ld, nil)
		return
	}

	assetDir := config.Current.Images.Kometa.AssetDirectory
	if assetDir == "" {
		logAction.SetError("Kometa asset directory not configured", "Set the Kometa asset directory in the configuration", nil)
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Resolve the path and ensure it stays within the asset directory (prevents traversal).
	cleanBase := filepath.Clean(assetDir)
	folderPath := filepath.Clean(filepath.Join(cleanBase, folder))
	fullPath := filepath.Clean(filepath.Join(folderPath, file))
	if !strings.HasPrefix(fullPath, cleanBase+string(os.PathSeparator)) {
		logAction.SetError("Invalid Kometa asset path", "Resolved path escapes the asset directory", map[string]any{"asset_id": assetID})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Prevent symlink escapes: do not allow the folder or file to be symlinks.
	if info, err := os.Lstat(folderPath); err != nil || info.Mode()&os.ModeSymlink != 0 {
		logAction.SetError("Invalid Kometa asset path", "Kometa asset folders must not be symlinks", map[string]any{"path": folderPath})
		httpx.SendResponse(w, ld, nil)
		return
	}
	if info, err := os.Lstat(fullPath); err != nil || info.Mode()&os.ModeSymlink != 0 {
		logAction.SetError("Invalid Kometa asset path", "Kometa asset files must not be symlinks", map[string]any{"path": fullPath})
		httpx.SendResponse(w, ld, nil)
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		logAction.SetError("Failed to read Kometa asset", "The asset file could not be read", map[string]any{"error": err.Error(), "path": fullPath})
		httpx.SendResponse(w, ld, nil)
		return
	}

	w.Header().Set("Content-Type", kometaContentType(filepath.Ext(fullPath)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func kometaContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
