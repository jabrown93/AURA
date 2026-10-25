package localimage

import (
	"aura/config"
	"aura/models"
	"testing"
)

func TestTemplatedPath(t *testing.T) {
	season, episode := 1, 5
	movie := models.MediaItem{Title: "Mission: Impossible", Year: 1996, TMDB_ID: "954", LibraryTitle: "Movies"}
	movieVars := nameTemplateVars(movie, models.ImageFile{Type: "poster"}, "8", "")
	showVars := nameTemplateVars(models.MediaItem{Title: "AC/DC Live", Year: 2020, TMDB_ID: "1", LibraryTitle: "TV Shows"},
		models.ImageFile{Type: "titlecard", SeasonNumber: &season, EpisodeNumber: &episode}, "9", "/tv/AC DC Live/Season 01/AC DC Live - S01E05 - Pilot.mkv")

	tests := []struct {
		name, template, folder, root string
		vars                         map[string]string
		want                         string
	}{
		{"in the item's folder", "folder", "/media/movies/Mission Impossible (1996)", "", movieVars, "/media/movies/Mission Impossible (1996)/folder.jpg"},
		{"Agregarr layout from the Path folder", "/{{LibraryTitle}}-{{LibraryID}}/{{Title}} ({{Year}}) tmdb-{{TMDBID}}/poster", "/images/movies/Mission Impossible (1996)", "/agregarr/plex-base-posters", movieVars,
			"/agregarr/plex-base-posters/Movies-8/Mission Impossible (1996) tmdb-954/poster.jpg"},
		{"a leading / without a Path stays in the folder", "/poster", "/media/movies/M", "", movieVars, "/media/movies/M/poster.jpg"},
		{"values can't make folders", "{{Title}}", "/tv/show", "", showVars, "/tv/show/ACDC Live.jpg"},
		{"titlecard names", "S{{SeasonNumber}}E{{EpisodeNumber}} - {{EpisodeFileName}}", "/tv/show/Season 01", "", showVars, "/tv/show/Season 01/S01E05 - AC DC Live - S01E05 - Pilot.jpg"},
		{".. can't leave the folder", "../../../etc/passwd", "/media/movies/M", "", movieVars, "/media/movies/M/etc/passwd.jpg"},
		{"an empty name uses the standard name", "{{Edition}}", "/media/movies/M", "", movieVars, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := templatedPath(tt.template, tt.folder, tt.root, tt.vars, ".jpg"); got != tt.want {
				t.Errorf("templatedPath = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultNameTemplatesKeepTheStandardNames(t *testing.T) {
	defaults := config.DefaultImageNameTemplates()
	season, episode := 1, 5
	show := models.MediaItem{Title: "Show", Year: 2020, LibraryTitle: "TV"}
	episodePath := "/tv/Show/Season 01/Show - S01E05 - Part: One.mkv"
	tests := []struct {
		template string
		image    models.ImageFile
		want     string
	}{
		{defaults.Poster, models.ImageFile{Type: "poster"}, "/tv/Show/poster.jpg"},
		{defaults.Backdrop, models.ImageFile{Type: "backdrop"}, "/tv/Show/backdrop.jpg"},
		{defaults.SeasonPoster, models.ImageFile{Type: "season_poster", SeasonNumber: &season}, "/tv/Show/season01-poster.jpg"},
		{defaults.SpecialSeasonPoster, models.ImageFile{Type: "special_season_poster"}, "/tv/Show/season-specials-poster.jpg"},
		// The episode's file name is kept as it is, so the title card matches its episode
		{defaults.Titlecard, models.ImageFile{Type: "titlecard", SeasonNumber: &season, EpisodeNumber: &episode}, "/tv/Show/Season 01/Show - S01E05 - Part: One-thumb.jpg"},
	}
	for _, tt := range tests {
		folder := "/tv/Show"
		if tt.image.Type == "titlecard" {
			folder = "/tv/Show/Season 01"
		}
		vars := nameTemplateVars(show, tt.image, "", episodePath)
		if got := templatedPath(tt.template, folder, "", vars, ".jpg"); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.image.Type, got, tt.want)
		}
	}
}
