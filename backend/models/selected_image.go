package models

import (
	"cmp"
	"math"
	"slices"
)

// IsKnownImageType reports whether aura knows how to apply an image of this type
func IsKnownImageType(imageType string) bool {
	switch imageType {
	case "poster", "backdrop", "season_poster", "titlecard":
		return true
	}
	return false
}

// ImageIsSelected reports whether an image of a saved set should be applied to the item.
// The image type must be selected, and the item must have the season or episode the image is for.
func ImageIsSelected(item MediaItem, selected SelectedTypes, image ImageFile) bool {
	switch image.Type {
	case "poster":
		return selected.Poster
	case "backdrop":
		return selected.Backdrop
	case "season_poster":
		if image.SeasonNumber == nil || !itemHasSeason(item, *image.SeasonNumber) {
			return false
		}
		if *image.SeasonNumber == 0 {
			return selected.SpecialSeasonPoster
		}
		return selected.SeasonPoster
	case "titlecard":
		return itemHasEpisode(item, image.SeasonNumber, image.EpisodeNumber) && selected.Titlecard
	}
	return false
}

func itemHasSeason(item MediaItem, seasonNumber int) bool {
	if item.Series == nil {
		return false
	}
	for _, season := range item.Series.Seasons {
		if season.SeasonNumber == seasonNumber {
			return true
		}
	}
	return false
}

// itemHasEpisode reports whether the item has the episode. A missing season or episode number matches any
func itemHasEpisode(item MediaItem, seasonNumber, episodeNumber *int) bool {
	if item.Series == nil {
		return false
	}
	for _, season := range item.Series.Seasons {
		if seasonNumber != nil && *seasonNumber != season.SeasonNumber {
			continue
		}
		for _, episode := range season.Episodes {
			if episodeNumber != nil && *episodeNumber != episode.EpisodeNumber {
				continue
			}
			return true
		}
	}
	return false
}

// SortImagesForApply returns the images in the order they are applied: the poster, the backdrop,
// then each season's poster followed by its titlecards by episode. Specials come after the other seasons.
func SortImagesForApply(images []ImageFile) []ImageFile {
	sorted := slices.Clone(images)
	slices.SortStableFunc(sorted, func(a, b ImageFile) int {
		return cmp.Or(
			cmp.Compare(imageTypeRank(a), imageTypeRank(b)),
			cmp.Compare(seasonRank(a), seasonRank(b)),
			cmp.Compare(seasonImageRank(a), seasonImageRank(b)),
			cmp.Compare(numberOrZero(a.EpisodeNumber), numberOrZero(b.EpisodeNumber)),
		)
	})
	return sorted
}

// imageTypeRank puts the poster first, then the backdrop, then the season images
func imageTypeRank(image ImageFile) int {
	switch image.Type {
	case "poster":
		return 0
	case "backdrop":
		return 1
	case "season_poster", "titlecard":
		return 2
	}
	return 3
}

// seasonRank orders the seasons, with specials after the other seasons
func seasonRank(image ImageFile) int {
	if image.SeasonNumber == nil {
		return 0
	}
	if *image.SeasonNumber == 0 {
		return math.MaxInt
	}
	return *image.SeasonNumber
}

// seasonImageRank puts a season's poster before its titlecards
func seasonImageRank(image ImageFile) int {
	if image.Type == "season_poster" {
		return 0
	}
	return 1
}

func numberOrZero(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
