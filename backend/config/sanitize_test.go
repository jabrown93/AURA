package config

import (
	"testing"
)

func webhookConfig(url string, headers map[string]string) *Config {
	return &Config{
		Notifications: Config_Notifications{
			Providers: []Config_Notification_Provider{
				{
					Provider: "Webhook",
					Enabled:  true,
					Webhook:  &Config_Notification_Webhook{URL: url, Headers: headers},
				},
			},
		},
	}
}

func TestSanitizeMasksWebhookHeaders(t *testing.T) {
	original := webhookConfig("https://hooks.example.com/notify", map[string]string{
		"Authorization": "Bearer supersecrettoken",
	})

	sanitized := original.SanitizeConfig(testContext())

	got := sanitized.Notifications.Providers[0].Webhook.Headers["Authorization"]
	if got == "Bearer supersecrettoken" {
		t.Fatal("Authorization header returned unmasked")
	}
	if got != MaskToken("Bearer supersecrettoken") {
		t.Errorf("header = %q, want %q", got, MaskToken("Bearer supersecrettoken"))
	}

	// The sanitized copy must not share the headers map with the live config, or masking
	// the copy would destroy the real credential.
	if original.Notifications.Providers[0].Webhook.Headers["Authorization"] != "Bearer supersecrettoken" {
		t.Error("sanitizing mutated the original config")
	}
}

func TestSanitizeLeavesWebhookURL(t *testing.T) {
	original := webhookConfig("https://hooks.example.com/notify", nil)

	sanitized := original.SanitizeConfig(testContext())

	if got := sanitized.Notifications.Providers[0].Webhook.URL; got != "https://hooks.example.com/notify" {
		t.Errorf("URL = %q, want it left intact so it round-trips on save", got)
	}
}
