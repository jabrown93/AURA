package plex

import (
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (c *Plex) RateMediaItem(ctx context.Context, item *models.MediaItem, rating float64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Rating Media Item %s", utils.MediaItemInfo(*item),
	), logging.LevelInfo)
	defer logAction.Complete()
	logAction.AppendResult("rating", rating)

	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex API request
	u, err := url.Parse(c.Config.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, ":", "rate")
	query := u.Query()
	query.Add("identifier", "com.plexapp.plugins.library")
	query.Add("key", item.RatingKey)
	query.Add("rating", fmt.Sprintf("%.1f", rating))
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, _, Err := makeRequest(ctx, c.Config, URL, "PUT", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	return Err
}
