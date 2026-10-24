package silo

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Silo) RefreshMediaItemMetadata(ctx context.Context, item *models.MediaItem, refreshRatingKey string, updateImage bool) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Refreshing Metadata for %s - Refresh Key: %s",
		utils.MediaItemInfo(*item),
		refreshRatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Construct the URL for the Silo server API request
	u, err := apiURL(s.Config.URL, "admin", "items", refreshRatingKey, "refresh-metadata")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	body, _ := json.Marshal(SiloRefreshRequest{Mode: "quick"})

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, s.Config, u.String(), "POST", body)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// Decode the Response
	var jobResp SiloJobResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &jobResp, "Silo Refresh Job Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	logAction.AppendResult("job_id", jobResp.ID)

	// Silo refreshes the item in a background job, so wait for it to finish
	jobResp, Err = waitForJob(ctx, s.Config, jobResp)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	if !jobResp.Terminal {
		logAction.AppendWarning("message", "Silo is still refreshing the item, the changes may not show yet")
		return logging.LogErrorInfo{}
	}
	if jobResp.State != "succeeded" {
		logAction.SetError("Silo failed to refresh the item", "Check the Silo server logs for more details",
			map[string]any{"job_id": jobResp.ID, "state": jobResp.State})
		return *logAction.Error
	}

	// Silo may change the Content ID of an item when it is refreshed
	// Get the latest Media Item details so the new Content ID and metadata are saved
	if refreshRatingKey == item.RatingKey {
		oldRatingKey := item.RatingKey
		_, Err = s.GetMediaItemDetails(ctx, item)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}
		if item.RatingKey != oldRatingKey {
			logAction.AppendResult("content_id_changed", map[string]any{"old": oldRatingKey, "new": item.RatingKey})
		}
	}

	return logging.LogErrorInfo{}
}

func waitForJob(ctx context.Context, msConfig config.Config_MediaServer, job SiloJobResponse) (SiloJobResponse, logging.LogErrorInfo) {
	// Check the job every second, for up to 30 seconds
	for attempt := 0; attempt < 30 && !job.Terminal; attempt++ {
		time.Sleep(time.Second)

		// Construct the URL for the Silo server API request
		u, err := apiURL(msConfig.URL, "admin", "jobs", job.ID)
		if err != nil {
			message, help, detail := logging.Error_BaseUrlParsing(err)
			return job, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
		}

		// Make the HTTP Request to Silo
		_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
		if Err.Message != "" {
			return job, Err
		}

		// Decode the Response
		Err = httpx.DecodeResponseToJSON(ctx, respBody, &job, "Silo Job Response")
		if Err.Message != "" {
			return job, Err
		}
	}
	return job, logging.LogErrorInfo{}
}
