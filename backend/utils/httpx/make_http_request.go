package httpx

import (
	"aura/logging"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var sensitiveURLParams = []string{"token", "key", "secret", "password", "auth", "sig"}

// WebhookSiteName marks a request to a user-configured webhook, whose URL is treated as a
// credential in full rather than just its query string.
const WebhookSiteName = "Webhook"

// redactURL blanks credentials carried in a URL. Plex sends its account token as an
// X-Plex-Token query parameter, and these URLs are logged on every transport error.
// hidePath drops the path as well, for endpoints where the path itself is the credential:
// a generic webhook is a user-supplied capability URL (the Slack shape puts the secret in
// /services/.../<secret>), so nothing past the host is safe to log. It stays on for the
// media-server and MediUX calls, whose paths name the failing endpoint and carry no secret.
func redactURL(raw string, hidePath bool) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<unparseable url>"
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
	}
	if hidePath {
		// Built from the safe components rather than blanked in place: an opaque URL such
		// as "https:secret" keeps the credential in parsed.Opaque, which String() emits
		// while ignoring Path, so clearing Path alone would leave the secret intact.
		// Validation only requires a non-empty URL, so that shape does reach here.
		if parsed.Host == "" {
			return parsed.Scheme + "://***"
		}
		return parsed.Scheme + "://" + parsed.Host + "/***"
	}
	query := parsed.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		for _, sensitive := range sensitiveURLParams {
			if strings.Contains(lowerKey, sensitive) {
				query.Set(key, "***")
				break
			}
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// redactErr strips credentials from an error message. http.NewRequest and http.Client.Do
// both return *url.Error, whose Error() embeds the full request URL, so redacting the
// separate "url" log field is not enough on its own.
//
// The embedded URL is rebuilt from the error rather than string-replaced: when the URL
// carries a password, net/http's stripPassword rewrites it to "user:***@host" before
// storing it, so it no longer matches raw and a substring replacement would silently do
// nothing while leaving any query-string credential exposed.
func redactErr(err error, raw string, hidePath bool) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Sprintf("%s %q: %s", urlErr.Op, redactURL(urlErr.URL, hidePath), redactErr(urlErr.Err, raw, hidePath))
	}
	return strings.ReplaceAll(err.Error(), raw, redactURL(raw, hidePath))
}

var sharedTransport = &http.Transport{
	TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
	ForceAttemptHTTP2:   true,
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
}

var sharedClient = &http.Client{
	Transport: sharedTransport,
	// no Timeout here; use request context timeout instead
}

// MakeHTTPRequest function to handle HTTP requests
func MakeHTTPRequest(ctx context.Context, url, method string, headers map[string]string, timeout int, body []byte, siteName string) (*http.Response, []byte, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Making %s request to %s", method, siteName), logging.LevelTrace)
	defer logAction.Complete()

	// Webhook endpoints are configured by the user and may carry their secret in the path.
	hidePath := siteName == WebhookSiteName

	// Create a context with a timeout
	timeoutInterval := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeoutInterval)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		logAction.SetError(fmt.Sprintf("Failed to create %s request to %s", method, siteName),
			"Check error and try again",
			map[string]any{
				"method": method,
				"url":    redactURL(url, hidePath),
				"error":  redactErr(err, url, hidePath),
			})
		return nil, nil, *logAction.Error
	}

	// Add a User-Agent header to the request
	req.Header.Set("User-Agent", "aura/1.0")
	req.Header.Set("X-Request", "mediux-aura")

	// Only header names are logged. Webhook headers are caller-supplied, so no list of
	// sensitive names can be complete - X-Webhook-Key and X-Hub-Signature-256 both carry
	// credentials and match no obvious pattern. Names alone are enough to debug with.
	for key, value := range headers {
		req.Header.Add(key, value)
		logAction.AppendResult("headers_added", key)
	}

	// Only set Content-Type to application/json if not already set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add common headers
	req.Header.Set("Connection", "keep-alive")

	// Send the HTTP request
	resp, err := sharedClient.Do(req)
	if err != nil {
		logAction.SetError(fmt.Sprintf("Failed to send %s request to %s", method, siteName),
			"Check error and try again",
			map[string]any{
				"method": method,
				"url":    redactURL(url, hidePath),
				"error":  redactErr(err, url, hidePath),
			})
		return nil, nil, *logAction.Error
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		logAction.SetError(fmt.Sprintf("Failed to read response body from %s", siteName),
			"Check error and try again",
			map[string]any{
				"method":      method,
				"url":         redactURL(url, hidePath),
				"error":       redactErr(err, url, hidePath),
				"status_code": resp.StatusCode,
			})
		return nil, nil, *logAction.Error
	}
	defer resp.Body.Close()

	// Add the Status Code the logging context Result
	logAction.AppendResult("status_code", resp.StatusCode)

	// Return the response
	return resp, respBody, logging.LogErrorInfo{}
}
