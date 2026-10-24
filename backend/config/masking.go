package config

import (
	"regexp"
	"strings"
)

func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) < 4 {
		return "***" + token[len(token)-1:]
	}
	return "***" + token[len(token)-4:]
}

func MaskWebhookURL(url string) string {
	if url == "" {
		return ""
	}

	// Split the URL by "/"
	parts := strings.Split(url, "/")
	if len(parts) < 6 {
		return url
	}

	// Get the webhook ID and token
	webhookID := parts[len(parts)-2]
	webhookToken := parts[len(parts)-1]

	// Mask the webhook ID - keep last 3 digits
	maskedID := "****" + webhookID[len(webhookID)-3:]

	// Mask the token - keep last 3 characters
	maskedToken := "***" + webhookToken[len(webhookToken)-3:]

	// Reconstruct the URL
	parts[len(parts)-2] = maskedID
	parts[len(parts)-1] = maskedToken

	return strings.Join(parts, "/")
}

// IsMaskedWebhook checks if the given string matches the masked webhook pattern.
func IsMaskedWebhook(s string) bool {
	var reMasked3 = regexp.MustCompile(`\*{4}[^/]{3}/\*{3}[^/]{3}$`)
	return reMasked3.MatchString(strings.TrimSpace(s))
}

func IsMaskedField(s string) bool {
	var reMaskedField = regexp.MustCompile(`\*{3}[^*]{1,}$`)
	return reMaskedField.MatchString(strings.TrimSpace(s))
}
