package httpx

import (
	"net/url"
	"strings"
)

// Query parameters that can hold secrets
func isSensitiveParam(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "key", "code", "state", "sig":
		return true
	}
	for _, part := range []string{"token", "password", "secret", "apikey", "api_key", "auth", "signature", "session"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

// RedactURL hides secrets in a URL, like query parameters and passwords, so it can be logged.
// If the URL can't be parsed, the query string is removed.
func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		before, _, _ := strings.Cut(rawURL, "?")
		return before
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), "REDACTED")
		}
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = RedactQuery(parsed.Query()).Encode()
	}
	return parsed.String()
}

// RedactQuery returns a copy of the query parameters with sensitive values hidden
func RedactQuery(values url.Values) url.Values {
	redacted := url.Values{}
	for name, list := range values {
		if isSensitiveParam(name) {
			redacted[name] = []string{"REDACTED"}
			continue
		}
		redacted[name] = list
	}
	return redacted
}

// RedactURLInText replaces a URL in a message, like a Go error, with its redacted form
func RedactURLInText(text, rawURL string) string {
	if rawURL == "" {
		return text
	}
	return strings.ReplaceAll(text, rawURL, RedactURL(rawURL))
}
