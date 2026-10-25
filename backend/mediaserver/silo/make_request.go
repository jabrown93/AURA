package silo

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

func makeRequest(ctx context.Context, msConfig config.Config_MediaServer, url string, method string, body []byte) (resp *http.Response, respBody []byte, Err logging.LogErrorInfo) {
	return makeRequestWithHeaders(ctx, msConfig, url, method, body, nil)
}

// makeRequestWithHeaders is makeRequest with more headers
func makeRequestWithHeaders(ctx context.Context, msConfig config.Config_MediaServer, url string, method string, body []byte, extraHeaders map[string]string) (resp *http.Response, respBody []byte, Err logging.LogErrorInfo) {
	// Make the HTTP Headers for this request
	headers := make(map[string]string)
	headers = AddSiloAuthHeaders(msConfig, headers)
	for name, value := range extraHeaders {
		headers[name] = value
	}

	// Make the HTTP request to Silo
	// If Silo rate limits the request, wait for the Retry-After delay and try again
	for attempt := 1; ; attempt++ {
		resp, respBody, Err = httpx.MakeHTTPRequest(ctx, url, method, headers, 60, body, "Silo")
		if Err.Message != "" {
			return nil, nil, Err
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt > maxRateLimitRetries {
			break
		}
		delay := getRetryAfterDelay(resp)
		logging.LOGGER.Debug().Timestamp().Str("url", url).Int("attempt", attempt).
			Msgf("Silo rate limited the request, retrying in %s", delay)
		time.Sleep(delay)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Decode the Error Response from Silo
		var problem SiloProblemResponse
		_ = json.Unmarshal(respBody, &problem)

		message := fmt.Sprintf("Silo Server returned a %d status code", resp.StatusCode)
		if problem.Title != "" {
			message = fmt.Sprintf("%s (%s)", message, problem.Title)
		}
		help := "Check the response from the server for more details"
		if problem.Detail != "" {
			help = problem.Detail
		}
		return nil, nil, logging.LogErrorInfo{
			Message: message,
			Help:    help,
			Detail:  map[string]any{"status_code": resp.StatusCode, "error_body": string(respBody)},
		}
	}

	return resp, respBody, logging.LogErrorInfo{}
}

func AddSiloAuthHeaders(msConfig config.Config_MediaServer, headers map[string]string) map[string]string {
	headers["Authorization"] = fmt.Sprintf("Bearer %s", msConfig.ApiToken)
	headers["Accept"] = "application/json"
	if msConfig.UserID != "" {
		headers["X-Profile-Id"] = msConfig.UserID
	}
	return headers
}

func apiURL(baseURL string, pathParts ...string) (*url.URL, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(append([]string{u.Path, "api", "v2"}, pathParts...)...)
	return u, nil
}

const maxRateLimitRetries = 10

func getRetryAfterDelay(resp *http.Response) time.Duration {
	seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || seconds < 1 {
		return time.Second
	}
	if seconds > 30 {
		return 30 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func isAuthError(Err logging.LogErrorInfo) bool {
	statusCode, _ := Err.Detail["status_code"].(int)
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func isNotFoundError(Err logging.LogErrorInfo) bool {
	statusCode, _ := Err.Detail["status_code"].(int)
	return statusCode == http.StatusNotFound
}
