package utils

import (
	"aura/models"
	"fmt"
)

// RatingKeyNotFoundMessage builds an actionable message and help string for when a
// media item's rating key cannot be determined for an image. For season posters and
// titlecards the most common cause is that the season/episode does not (yet) exist on
// the media server, so the message names the missing season/episode.
func RatingKeyNotFoundMessage(imageFile models.ImageFile) (message string, help string) {
	switch imageFile.Type {
	case "season_poster", "special_season_poster":
		if imageFile.SeasonNumber != nil {
			return fmt.Sprintf("Season %d not found on the media server for this show", *imageFile.SeasonNumber),
				"The season does not exist on the media server yet. It will be applied automatically once the season is added, if the set is saved with Auto Download enabled."
		}
	case "titlecard":
		if imageFile.SeasonNumber != nil && imageFile.EpisodeNumber != nil {
			return fmt.Sprintf("Season %d Episode %d not found on the media server for this show", *imageFile.SeasonNumber, *imageFile.EpisodeNumber),
				"The episode does not exist on the media server yet. It will be applied automatically once the episode is added, if the set is saved with Auto Download enabled."
		}
	}
	return "Failed to determine Rating Key for Media Item", "Ensure the Media Item and Image File data are correct"
}
