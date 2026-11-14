package autodownload

import (
	"testing"

	"aura/models"
)

// Regression test for the drifted season-poster gate in handleShow: it previously admitted an
// image when either SeasonPoster or SpecialSeasonPoster was selected, so a set selecting only
// one of the two downloaded both kinds.
func TestSeasonPosterSelected(t *testing.T) {
	seasonNumber := func(n int) *int { return &n }

	tests := []struct {
		name          string
		selectedTypes models.SelectedTypes
		seasonNumber  *int
		want          bool
	}{
		{"season 0 with special selected", models.SelectedTypes{SpecialSeasonPoster: true}, seasonNumber(0), true},
		{"season 0 with only regular selected", models.SelectedTypes{SeasonPoster: true}, seasonNumber(0), false},
		{"season 1 with regular selected", models.SelectedTypes{SeasonPoster: true}, seasonNumber(1), true},
		{"season 1 with only special selected", models.SelectedTypes{SpecialSeasonPoster: true}, seasonNumber(1), false},
		{"nil season with both selected", models.SelectedTypes{SeasonPoster: true, SpecialSeasonPoster: true}, nil, false},
		{"season 1 with neither selected", models.SelectedTypes{}, seasonNumber(1), false},
	}

	item := models.MediaItem{Type: "show", Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{SeasonNumber: 0}, {SeasonNumber: 1}}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := models.ImageFile{Type: "season_poster", SeasonNumber: tt.seasonNumber}
			if got := models.ImageIsSelected(item, tt.selectedTypes, image); got != tt.want {
				t.Errorf("ImageIsSelected() = %v, want %v", got, tt.want)
			}
		})
	}
}
