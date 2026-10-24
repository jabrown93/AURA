package silo

import (
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"encoding/json"
	"fmt"
	"math"
)

func (s *Silo) RateMediaItem(ctx context.Context, item *models.MediaItem, rating float64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Rating Media Item %s", utils.MediaItemInfo(*item),
	), logging.LevelInfo)
	defer logAction.Complete()
	logAction.AppendResult("rating", rating)

	Err = logging.LogErrorInfo{}

	// The rating is on a scale of 10, but Silo uses whole stars from 1 to 5
	stars := min(int(math.Round(rating/2)), 5)
	logAction.AppendResult("silo_stars", stars)

	// Construct the URL for the Silo server API request
	u, err := apiURL(s.Config.URL, "ratings", item.RatingKey)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}

	// A rating of 0 removes the rating from Silo
	method := "DELETE"
	var body []byte
	if stars >= 1 {
		method = "PUT"
		body, _ = json.Marshal(SiloRatingRequest{Rating: stars})
	}

	// Make the HTTP Request to Silo
	_, _, Err = makeRequest(ctx, s.Config, u.String(), method, body)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}
