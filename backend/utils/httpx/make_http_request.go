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
// credential in full and kept out of logs entirely.
const WebhookSiteName = "Webhook"

// redactedWebhookURL replaces a webhook URL entirely in logs.
//
// A generic webhook is a user-supplied capability URL, and every component has turned out to
// be able to carry the secret: the query string, the path (Slack's /services/.../<secret>),
// the opaque remainder of "https:secret", and a DNS label such as
// https://<token>.hooks.example. Validation only requires the URL to be non-empty, so all of
// those shapes reach the log. Rather than keep discovering components to blank, none of it
// is logged - the site name already says which provider failed.
const redactedWebhookURL = "<webhook url redacted>"

// redactURL blanks credentials carried in a URL. Plex sends its account token as an
// X-Plex-Token query parameter, and these URLs are logged on every transport error.
// redactAll drops the URL completely, for webhooks. Media-server and MediUX URLs keep their
// host and path: they name the failing endpoint and carry no secret.
func redactURL(raw string, redactAll bool) string {
	if redactAll {
		return redactedWebhookURL
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<unparseable url>"
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
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
func redactErr(err error, raw string, redactAll bool) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Sprintf("%s %q: %s", urlErr.Op, redactURL(urlErr.URL, redactAll), redactErr(urlErr.Err, raw, redactAll))
	}
	msg := strings.ReplaceAll(err.Error(), raw, redactURL(raw, redactAll))
	if !redactAll {
		return msg
	}
	// DNS and dial failures name the host on their own, without the surrounding URL, so
	// replacing the URL is not enough when the host itself is the credential.
	if parsed, parseErr := url.Parse(raw); parseErr == nil {
		for _, host := range []string{parsed.Host, parsed.Hostname()} {
			if host != "" {
				msg = strings.ReplaceAll(msg, host, "***")
			}
		}
	}
	return msg
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
	redactAll := siteName == WebhookSiteName

	// Bound the request by the timeout while still honoring the caller's context, so an
	// abandoned caller (cancelled request, shutdown) cancels the outbound call instead of
	// leaving it to run out its full timeout.
	timeoutInterval := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeoutInterval)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		logAction.SetError(fmt.Sprintf("Failed to create %s request to %s", method, siteName),
			"Check error and try again",
			map[string]any{
				"method": method,
				"url":    redactURL(url, redactAll),
				"error":  redactErr(err, url, redactAll),
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
				"url":    redactURL(url, redactAll),
				"error":  redactErr(err, url, redactAll),
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
				"url":         redactURL(url, redactAll),
				"error":       redactErr(err, url, redactAll),
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
