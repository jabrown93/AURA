package silo

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
)

type Silo struct {
	Config config.Config_MediaServer
}

func (s *Silo) GetAdminUser(ctx context.Context, msConfig config.Config_MediaServer) (userID string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting Admin User for Silo Media Server", logging.LevelDebug)
	defer logAction.Complete()

	userID = ""
	Err = logging.LogErrorInfo{}

	// Clear the Profile ID so a previous value is not sent with these requests
	msConfig.UserID = ""

	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "account", "me")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return userID, *logAction.Error
	}

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return userID, *logAction.Error
	}

	// Decode the Response
	var accountResp SiloAccountResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &accountResp, "Silo Account Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return userID, *logAction.Error
	}

	// Applying images and refreshing metadata require an admin account
	if accountResp.Role != "admin" {
		logAction.SetError("Silo API Key does not belong to an admin account", "Create the API Key from an admin account on the Silo server", map[string]any{
			"username": accountResp.Username,
			"role":     accountResp.Role,
		})
		return userID, *logAction.Error
	}

	// Construct the URL for the Silo server API request
	u, _ = apiURL(msConfig.URL, "profiles")

	// Make the HTTP Request to Silo
	_, respBody, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return userID, *logAction.Error
	}

	// Decode the Response
	var profilesResp SiloProfilesResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &profilesResp, "Silo Profiles Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return userID, *logAction.Error
	}

	// Find the Primary Profile ID
	for _, profile := range profilesResp.Items {
		if profile.IsPrimary {
			userID = profile.ID
			logAction.AppendResult("admin_user_id", profile.ID)
			return userID, logging.LogErrorInfo{}
		}
	}

	logAction.SetError("No primary profile found on Silo server", "Ensure that the admin account has a primary profile", map[string]any{
		"username": accountResp.Username,
	})
	return userID, *logAction.Error
}
