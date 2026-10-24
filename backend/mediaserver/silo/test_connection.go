package silo

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"time"
)

func (s *Silo) TestConnection(ctx context.Context, msConfig config.Config_MediaServer) (connectionOK bool, serverName string, serverVersion string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Silo: Checking Media Server Connection", logging.LevelDebug)
	defer logAction.Complete()

	connectionOK = false
	serverName = ""
	serverVersion = ""
	Err = logging.LogErrorInfo{}

	var lastErr logging.LogErrorInfo

	// Try up to 3 times with a 5 second delay between attempts
	for attempt := 1; attempt <= 3; attempt++ {
		attemptAction := logAction.AddSubAction(fmt.Sprintf("Attempt %d to connect to Silo", attempt), logging.LevelTrace)
		serverName, serverVersion, lastErr = getServerInfo(ctx, msConfig)
		if lastErr.Message == "" {
			connectionOK = true
			return connectionOK, serverName, serverVersion, logging.LogErrorInfo{}
		}
		attemptAction.AppendWarning(fmt.Sprintf("attempt_%d", attempt), map[string]any{"error": lastErr.Message})

		// If the API Key was rejected, there is no need to retry
		if isAuthError(lastErr) {
			lastErr.Help = "Ensure the API Key is valid and has not been revoked"
			break
		}
		if attempt < 3 {
			time.Sleep(5 * time.Second)
		}
	}

	// All attempts failed
	return connectionOK, "", "", lastErr
}

func getServerInfo(ctx context.Context, msConfig config.Config_MediaServer) (serverName string, serverVersion string, Err logging.LogErrorInfo) {
	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "system", "info")
	if err != nil {
		message, help, detail := logging.Error_BaseUrlParsing(err)
		return "", "", logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
	}

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return "", "", Err
	}

	// Decode the Response
	var infoResp SiloSystemInfoResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &infoResp, "Silo System Info Response")
	if Err.Message != "" {
		return "", "", Err
	}

	// If the server version is empty, consider the connection failed
	if infoResp.ServerVersion == "" {
		return "", "", logging.LogErrorInfo{
			Message: "Failed to retrieve Silo server version",
			Help:    "Ensure the URL points to a Silo server",
			Detail:  map[string]any{"URL": u.String()},
		}
	}

	// If the API version is not supported, consider the connection failed
	if infoResp.APIMajor != 2 {
		return "", "", logging.LogErrorInfo{
			Message: fmt.Sprintf("Unsupported Silo API version: %d", infoResp.APIMajor),
			Help:    "aura supports version 2 of the Silo API",
			Detail:  map[string]any{"api_major": infoResp.APIMajor, "server_version": infoResp.ServerVersion},
		}
	}

	// System Info does not require authentication, so check the API Key using the Account endpoint
	u, _ = apiURL(msConfig.URL, "account", "me")
	_, _, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return "", "", Err
	}

	// Get the server name from the Branding endpoint
	u, _ = apiURL(msConfig.URL, "theme", "branding")
	_, respBody, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return "", "", Err
	}

	// Decode the Response
	var brandingResp SiloBrandingResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &brandingResp, "Silo Branding Response")
	if Err.Message != "" {
		return "", "", Err
	}

	// If the server name is empty, default to Silo
	serverName = brandingResp.ServerName
	if serverName == "" {
		serverName = "Silo"
	}
	return serverName, infoResp.ServerVersion, logging.LogErrorInfo{}
}
