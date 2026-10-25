package localimage

import (
	"aura/config"
	"aura/models"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Characters a variable's value can't add to a file name, so a title can't make folders
var unsafeNameChars = regexp.MustCompile(`[/\\:*?"<>|]`)

// nameTemplate returns the name template for the image type, and its standard name
func nameTemplate(imageType string) (template, standard string) {
	templates := config.Current.Images.SaveImagesLocally.NameTemplates
	defaults := config.DefaultImageNameTemplates()
	switch imageType {
	case "poster":
		template, standard = templates.Poster, defaults.Poster
	case "backdrop":
		template, standard = templates.Backdrop, defaults.Backdrop
	case "season_poster":
		template, standard = templates.SeasonPoster, defaults.SeasonPoster
	case "special_season_poster":
		template, standard = templates.SpecialSeasonPoster, defaults.SpecialSeasonPoster
	case "titlecard":
		template, standard = templates.Titlecard, defaults.Titlecard
	}
	if template == "" {
		template = standard
	}
	return template, standard
}

// nameTemplateVars are the values of the name template variables for an image
func nameTemplateVars(item models.MediaItem, imageFile models.ImageFile, libraryID, episodePath string) map[string]string {
	twoDigits := func(n *int) string {
		if n == nil {
			return ""
		}
		return fmt.Sprintf("%02d", *n)
	}
	episodeFileName := path.Base(episodePath)
	if episodePath == "" {
		episodeFileName = ""
	}
	return map[string]string{
		"Title":           item.Title,
		"Year":            fmt.Sprintf("%d", item.Year),
		"TMDBID":          item.TMDB_ID,
		"LibraryTitle":    item.LibraryTitle,
		"LibraryID":       libraryID,
		"Edition":         item.Edition,
		"SeasonNumber":    twoDigits(imageFile.SeasonNumber),
		"EpisodeNumber":   twoDigits(imageFile.EpisodeNumber),
		"EpisodeFileName": strings.TrimSuffix(episodeFileName, path.Ext(episodeFileName)),
	}
}

// templatedPath is where an image with a name template is saved. The template is placed in folder,
// or from root when it starts with / and root is set. Empty when the template names nothing.
func templatedPath(template, folder, root string, vars map[string]string, ext string) string {
	name := template
	for key, value := range vars {
		// The episode's file name is kept as it is, so a title card matches its episode
		if key != "EpisodeFileName" {
			value = strings.TrimSpace(unsafeNameChars.ReplaceAllString(value, ""))
		}
		name = strings.ReplaceAll(name, "{{"+key+"}}", value)
	}
	// Cleaning from / keeps the name in its folder
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "" || name == "." {
		return ""
	}
	base := folder
	if strings.HasPrefix(template, "/") && root != "" {
		base = root
	}
	return path.Join(base, name) + ext
}
