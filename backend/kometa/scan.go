package kometa

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ScannedAsset is a single recognized asset file within a Kometa asset folder.
type ScannedAsset struct {
	FileName string    // base file name, e.g. "poster.jpg"
	Type     string    // internal image type: poster, backdrop, season_poster, special_season_poster, titlecard
	Season   *int      // set for season_poster, special_season_poster, titlecard
	Episode  *int      // set for titlecard
	ModTime  time.Time // file modification time
}

// ScannedFolder is one item-level directory in the asset directory and its recognized assets.
type ScannedFolder struct {
	Name         string         // folder base name (the Kometa ASSET_NAME); used for title/tmdb matching
	RelDir       string         // path relative to the asset directory: "<name>" (flat) or "<subfolder>/<name>"
	Assets       []ScannedAsset // recognized asset files
	Unrecognized int            // count of files that did not match any known asset name
}

// asset-name classification regexes (case-insensitive; extension validated separately).
var (
	kometaExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

	rePoster     = regexp.MustCompile(`(?i)^poster$`)
	reBackground = regexp.MustCompile(`(?i)^background$`)
	reSeason     = regexp.MustCompile(`(?i)^season(\d{1,3})$`)
	reEpisode    = regexp.MustCompile(`(?i)^s(\d{1,3})e(\d{1,3})$`)
)

// Scan discovers item-level asset folders under the asset directory and classifies their
// recognized asset files. It reads only names and modification times; image bytes are read
// later, one file at a time, during upload.
//
// subfolders is the set of per-library subfolders (relative to assetDir) configured via
// Config_Kometa.LibraryAssetFolders. When non-empty, Scan descends one level into each
// subfolder to find item folders (RelDir = "<subfolder>/<item>"). It still scans the asset
// root one level deep for the flat layout (RelDir = "<item>"), but skips any root entry that
// is the first segment of a configured subfolder so a library folder is never mistaken for an
// item folder.
func Scan(assetDir string, subfolders []string) ([]ScannedFolder, error) {
	// Distinct, non-empty subfolders and the set of root-level names they occupy.
	seen := make(map[string]bool)
	skipRoot := make(map[string]bool)
	cleaned := make([]string, 0, len(subfolders))
	for _, sub := range subfolders {
		sub = strings.Trim(strings.ReplaceAll(sub, "\\", "/"), "/")
		if sub == "" || seen[sub] {
			continue
		}
		seen[sub] = true
		cleaned = append(cleaned, sub)
		skipRoot[strings.SplitN(sub, "/", 2)[0]] = true
	}

	folders := make([]ScannedFolder, 0)

	// Flat layout: item folders directly under the asset root.
	rootEntries, err := os.ReadDir(assetDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range rootEntries {
		if !entry.IsDir() || skipRoot[entry.Name()] {
			continue
		}
		folders = append(folders, scanAssetFolder(assetDir, entry.Name()))
	}

	// Per-library layout: item folders one level inside each configured subfolder.
	for _, sub := range cleaned {
		subEntries, err := os.ReadDir(filepath.Join(assetDir, filepath.FromSlash(sub)))
		if err != nil {
			// Subfolder does not exist yet or is unreadable; nothing to import from it.
			continue
		}
		for _, entry := range subEntries {
			if !entry.IsDir() {
				continue
			}
			folders = append(folders, scanAssetFolder(assetDir, sub+"/"+entry.Name()))
		}
	}

	return folders, nil
}

// scanAssetFolder reads a single item-level asset folder at relDir (relative to assetDir) and
// classifies its files. relDir uses forward slashes; Name is its base segment.
func scanAssetFolder(assetDir, relDir string) ScannedFolder {
	folder := ScannedFolder{Name: path.Base(relDir), RelDir: relDir}

	files, err := os.ReadDir(filepath.Join(assetDir, filepath.FromSlash(relDir)))
	if err != nil {
		// Unreadable folder: record it as having no assets and move on.
		return folder
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		asset, ok := classifyAsset(file.Name())
		if !ok {
			folder.Unrecognized++
			continue
		}
		if info, err := file.Info(); err == nil {
			asset.ModTime = info.ModTime()
		}
		folder.Assets = append(folder.Assets, asset)
	}

	return folder
}

// classifyAsset maps a file name to an internal asset type, or ok=false if it is not a
// recognized Kometa asset file.
func classifyAsset(fileName string) (ScannedAsset, bool) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if !kometaExtensions[ext] {
		return ScannedAsset{}, false
	}
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	switch {
	case rePoster.MatchString(base):
		return ScannedAsset{FileName: fileName, Type: "poster"}, true
	case reBackground.MatchString(base):
		return ScannedAsset{FileName: fileName, Type: "backdrop"}, true
	case reSeason.MatchString(base):
		n, _ := strconv.Atoi(reSeason.FindStringSubmatch(base)[1])
		season := n
		if n == 0 {
			return ScannedAsset{FileName: fileName, Type: "special_season_poster", Season: &season}, true
		}
		return ScannedAsset{FileName: fileName, Type: "season_poster", Season: &season}, true
	case reEpisode.MatchString(base):
		m := reEpisode.FindStringSubmatch(base)
		s, _ := strconv.Atoi(m[1])
		e, _ := strconv.Atoi(m[2])
		return ScannedAsset{FileName: fileName, Type: "titlecard", Season: &s, Episode: &e}, true
	default:
		return ScannedAsset{}, false
	}
}
