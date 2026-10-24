package config

import (
	"aura/logging"
	"context"
	"fmt"
	"os"
	"strings"
)

// Fork: Kometa asset-directory mode (Plex only) and the fork-only background jobs.

// ForkJobDefaults are the schedules of the fork-only jobs registered in jobs/fork_jobs.go.
var ForkJobDefaults = struct {
	KometaImport JobSettingDefault
	AnidbRefresh JobSettingDefault
}{
	KometaImport: JobSettingDefault{Enabled: false, Cron: "0 3 * * *"},
	AnidbRefresh: JobSettingDefault{Enabled: true, Cron: "0 4 * * 1"},
}

type Config_Kometa struct {
	Enabled              bool              `json:"enabled" yaml:"Enabled"`                                                 // Whether to write downloaded images into the Kometa asset directory using Kometa naming conventions. Plex exclusive feature.
	AssetDirectory       string            `json:"asset_directory,omitempty" yaml:"AssetDirectory,omitempty"`              // Path to the Kometa asset directory (the same directory Kometa reads assets from). Uses the folder-per-item (asset_folders: true) layout.
	LibraryAssetFolders  map[string]string `json:"library_asset_folders,omitempty" yaml:"LibraryAssetFolders,omitempty"`   // Optional per-library subfolder (relative to AssetDirectory) keyed by Plex library title. When set, a library's assets are written to <AssetDirectory>/<subfolder>/<item>/... instead of directly under AssetDirectory. Must match the per-library asset_directory configured in Kometa itself. A library with no entry keeps writing flat to AssetDirectory.
	SonarrRadarrFallback bool              `json:"sonarr_radarr_fallback,omitempty" yaml:"SonarrRadarrFallback,omitempty"` // When a media-server lookup fails (e.g. Plex returns a 404) but the item exists in Sonarr/Radarr, still write the downloaded images into the Kometa asset directory, deriving the asset folder name from the Sonarr/Radarr path. Requires Enabled. Plex exclusive feature.
}

// SubfolderFor returns the Kometa asset subfolder configured for the given library title,
// relative to AssetDirectory. It returns "" when no per-library subfolder is configured, in
// which case assets are written directly under AssetDirectory (the flat, backward-compatible
// layout). The value is cleaned to a safe relative path; any absolute or parent-escaping
// value collapses to "" so a lookup can never redirect writes outside AssetDirectory.
func (k Config_Kometa) SubfolderFor(libraryTitle string) string {
	return SanitizeKometaSubfolder(k.LibraryAssetFolders[libraryTitle])
}

// SanitizeKometaSubfolder normalizes a per-library asset subfolder into a safe path relative
// to the Kometa asset directory. It trims whitespace and slashes, converts backslashes to
// forward slashes, drops "." and ".." segments, and returns "" for anything that is empty,
// absolute, or would escape the asset directory.
func SanitizeKometaSubfolder(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Normalize Windows separators so path.Clean sees a single-style path.
	s = strings.ReplaceAll(s, "\\", "/")
	// Reject absolute paths outright; they must not redirect writes outside AssetDirectory.
	if strings.HasPrefix(s, "/") {
		return ""
	}
	segments := make([]string, 0)
	for _, seg := range strings.Split(s, "/") {
		seg = strings.TrimSpace(seg)
		switch seg {
		case "", ".", "..":
			// Skip empty and relative segments; ".." must never climb out of AssetDirectory.
			if seg == ".." {
				return ""
			}
			continue
		default:
			segments = append(segments, seg)
		}
	}
	return strings.Join(segments, "/")
}

// validateKometa checks Kometa mode settings and normalizes per-library subfolders in place,
// so the write and import paths agree on the same value.
func validateKometa(ctx context.Context, k *Config_Kometa, msConfig Config_MediaServer) bool {
	_, logAction := logging.AddSubActionToContext(ctx, "Validating Kometa Config", logging.LevelTrace)
	defer logAction.Complete()

	if !k.Enabled {
		return true
	}
	if msConfig.Type != "Plex" {
		logAction.SetError("Images.Kometa is only supported for Plex media servers", "Disable Kometa mode or switch to a Plex media server", nil)
		return false
	}
	if k.AssetDirectory == "" {
		logAction.SetError("Images.Kometa.AssetDirectory is required when Kometa mode is enabled", "Set AssetDirectory to the path Kometa reads assets from", nil)
		return false
	}
	if info, err := os.Stat(k.AssetDirectory); err != nil || !info.IsDir() {
		logAction.SetError(
			fmt.Sprintf("Images.Kometa.AssetDirectory '%s' does not exist or is not a directory", k.AssetDirectory),
			"Ensure the Kometa asset directory is mounted into the container at this path", nil)
		return false
	}
	for title, raw := range k.LibraryAssetFolders {
		if strings.TrimSpace(raw) == "" {
			delete(k.LibraryAssetFolders, title)
			continue
		}
		sub := SanitizeKometaSubfolder(raw)
		if sub == "" {
			logAction.SetError(
				fmt.Sprintf("Images.Kometa.LibraryAssetFolders['%s']: '%s' is not a valid relative subfolder", title, raw),
				"Use a path relative to the Kometa asset directory (no leading '/' and no '..')", nil)
			return false
		}
		k.LibraryAssetFolders[title] = sub
	}
	return true
}
