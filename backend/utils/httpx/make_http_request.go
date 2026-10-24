package httpx

import (
	"aura/config"
	"aura/logging"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

	// Create a context with a timeout
	timeoutInterval := time.Duration(timeout) * time.Second
	// Fork: derive from the caller's context so a cancelled caller cancels the request.
	ctx, cancel := context.WithTimeout(ctx, timeoutInterval)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		logAction.SetError(fmt.Sprintf("Failed to create %s request to %s", method, siteName),
			"Check error and try again",
			map[string]any{
				"method": method,
				"url":    RedactURL(url),
				"error":  RedactURLInText(err.Error(), url),
			})
		return nil, nil, *logAction.Error
	}

	// Add a User-Agent header to the request
	req.Header.Set("User-Agent", "aura/1.0")
	req.Header.Set("X-Request", "mediux-aura")

	possibleSensitiveHeaders := []string{
		"authorization",
		"api-key",
		"x-api-key",
		"x-auth-token",
		"x-access-token",
		"access-token",
		"token",
		"authentication",
	}

	if len(headers) > 0 {
		for key, value := range headers {
			req.Header.Add(key, value)
			maskedValue := value
			lowerKey := strings.ToLower(key)
			for _, sensitive := range possibleSensitiveHeaders {
				if strings.Contains(lowerKey, sensitive) {
					maskedValue = config.MaskToken(value)
					break
				}
			}
			if strings.ToLower(value) != "aura" {
				logAction.AppendResult("headers_added", map[string]any{key: maskedValue})
			}
		}
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
				"url":    RedactURL(url),
				"error":  RedactURLInText(err.Error(), url),
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
				"url":         RedactURL(url),
				"error":       RedactURLInText(err.Error(), url),
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
