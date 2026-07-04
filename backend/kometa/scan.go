package kometa

import (
	"os"
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

// ScannedFolder is one depth-1 directory in the asset directory and its recognized assets.
type ScannedFolder struct {
	Name         string         // folder base name (the Kometa ASSET_NAME)
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

// Scan walks the asset directory one level deep and classifies the recognized asset files
// in each folder. It reads only names and modification times; image bytes are read later,
// one file at a time, during upload.
func Scan(assetDir string) ([]ScannedFolder, error) {
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		return nil, err
	}

	folders := make([]ScannedFolder, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := ScannedFolder{Name: entry.Name()}

		files, err := os.ReadDir(filepath.Join(assetDir, entry.Name()))
		if err != nil {
			// Unreadable subfolder: record it as having no assets and move on.
			folders = append(folders, folder)
			continue
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

		folders = append(folders, folder)
	}

	return folders, nil
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
