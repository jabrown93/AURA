// Package kometa implements AURA's Kometa asset-directory integration: importing
// existing Kometa assets from disk, uploading them to Plex, and registering them in the
// database. The write side (saving downloaded images into the Kometa asset directory)
// lives in the plex media-server package.
package kometa

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// SetIDPrefix marks a PosterSet as a synthetic set created by a Kometa asset import.
	// It is used to distinguish these sets from MediUX-sourced sets so that MediUX-only
	// operations (re-fetching a set by ID, auto-download, force-check) skip them.
	SetIDPrefix = "kometa-"

	// ImageIDPrefix marks an ImageFile as a locally-imported Kometa asset. The remainder
	// of the ID is "<assetFolder>/<fileName>" relative to the Kometa asset directory.
	ImageIDPrefix = "kometa|"
)

// IsKometaSetID reports whether a poster set ID belongs to a Kometa import.
func IsKometaSetID(id string) bool {
	return strings.HasPrefix(id, SetIDPrefix)
}

// IsKometaImageID reports whether an image ID refers to a locally-imported Kometa asset.
func IsKometaImageID(id string) bool {
	return strings.HasPrefix(id, ImageIDPrefix)
}

// KometaImageRelPath returns the "<assetFolder>/<fileName>" path (relative to the asset
// directory) encoded in a Kometa image ID, or "" if the ID is not a Kometa image ID.
func KometaImageRelPath(id string) string {
	if !IsKometaImageID(id) {
		return ""
	}
	return strings.TrimPrefix(id, ImageIDPrefix)
}

// setIDForItem builds a deterministic synthetic set ID for a matched media item so that
// re-running an import upserts the same set instead of creating duplicates.
func setIDForItem(tmdbID, libraryTitle string) string {
	return fmt.Sprintf("%s%s-%s", SetIDPrefix, tmdbID, slug(libraryTitle))
}

// imageIDForAsset builds the ImageFile ID for an imported asset.
func imageIDForAsset(folderName, fileName string) string {
	return ImageIDPrefix + folderName + "/" + fileName
}

var slugStripper = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// slug reduces a string to lowercase alphanumerics joined by dashes (used inside set IDs).
func slug(s string) string {
	s = strings.ToLower(s)
	s = slugStripper.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// normalizeTitle mirrors the media-server cache's title comparison: lowercase with common
// punctuation removed. It also drops filesystem-illegal characters (/, \, <, >, ", |, *)
// because on-disk asset folder names are sanitized while titles are not; without this a
// collection like "Alien / Predator" could never match its folder "Alien  Predator".
// Used to match Kometa asset folder names to collection titles.
func normalizeTitle(input string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(input) {
		switch r {
		case '-', '_', '.', ',', ':', ';', '!', '?', '\'', '(', ')', '[', ']', '{', '}',
			'/', '\\', '<', '>', '"', '|', '*':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

var (
	tmdbHintRegex  = regexp.MustCompile(`\{tmdb-(\d+)\}`)
	titleYearRegex = regexp.MustCompile(`^(.*?)\s*\((\d{4})\)`)
)

// parseTMDBHint extracts a TMDB ID from a "{tmdb-12345}" hint in a folder name, if present.
func parseTMDBHint(folderName string) (string, bool) {
	m := tmdbHintRegex.FindStringSubmatch(folderName)
	if len(m) == 2 {
		return m[1], true
	}
	return "", false
}

// parseTitleYear extracts the title and year from a "Title (Year)" folder name. Returns
// ok=false when no "(YYYY)" is present.
func parseTitleYear(folderName string) (title string, year int, ok bool) {
	m := titleYearRegex.FindStringSubmatch(folderName)
	if len(m) != 3 {
		return "", 0, false
	}
	title = strings.TrimSpace(m[1])
	// m[2] is always 4 digits, so this parse cannot fail.
	for _, r := range m[2] {
		year = year*10 + int(r-'0')
	}
	return title, year, true
}
