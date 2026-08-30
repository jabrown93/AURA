package routes_validation

import (
	"aura/config"
	"testing"
)

func webhookProvider(url string, headers map[string]string) config.Config_Notification_Provider {
	return config.Config_Notification_Provider{
		Provider: "Webhook",
		Enabled:  true,
		Webhook:  &config.Config_Notification_Webhook{URL: url, Headers: headers},
	}
}

func withProviders(t *testing.T, providers ...config.Config_Notification_Provider) {
	t.Helper()
	previous := config.Current.Notifications.Providers
	config.Current.Notifications.Providers = providers
	t.Cleanup(func() { config.Current.Notifications.Providers = previous })
}

// Several webhook providers can share a URL with different credentials. A mismatch against
// the first one means "keep looking", not "reject", or testing the second is impossible.
func TestUnmaskWebhookHeadersSearchesPastAMismatch(t *testing.T) {
	const sharedURL = "https://hooks.example.com/notify"
	const firstSecret = "Bearer alpha-1111"
	const secondSecret = "Bearer beta-2222"

	// Distinct endings, so the two masks differ and the second is genuinely identifiable.
	if config.MaskToken(firstSecret) == config.MaskToken(secondSecret) {
		t.Fatal("test needs two secrets whose masks differ")
	}

	withProviders(t,
		webhookProvider(sharedURL, map[string]string{"Authorization": firstSecret}),
		webhookProvider(sharedURL, map[string]string{"Authorization": secondSecret}),
	)

	submitted := &config.Config_Notification_Webhook{
		URL:     sharedURL,
		Headers: map[string]string{"Authorization": config.MaskToken(secondSecret)},
	}

	if !unmaskWebhookHeaders(submitted) {
		t.Fatal("second provider's masked header should resolve")
	}
	if got := submitted.Headers["Authorization"]; got != secondSecret {
		t.Errorf("header = %q, want %q", got, secondSecret)
	}
}

// A mask that matches no stored provider must be rejected, and must not leave the map
// half-restored with another provider's value.
func TestUnmaskWebhookHeadersRejectsUnknownMask(t *testing.T) {
	const sharedURL = "https://hooks.example.com/notify"

	withProviders(t, webhookProvider(sharedURL, map[string]string{"Authorization": "Bearer stored-secret"}))

	masked := config.MaskToken("Bearer something-else")
	submitted := &config.Config_Notification_Webhook{
		URL:     sharedURL,
		Headers: map[string]string{"Authorization": masked},
	}

	if unmaskWebhookHeaders(submitted) {
		t.Fatal("a mask matching no stored value must be rejected")
	}
	if got := submitted.Headers["Authorization"]; got != masked {
		t.Errorf("header = %q, want it left untouched", got)
	}
}

// Short values mask to "***" plus a single character, which a fixed three-character suffix
// comparison could never match.
func TestMaskedFieldMatchesHandlesShortValues(t *testing.T) {
	for _, value := range []string{"a", "ab", "abc", "abcd", "long-secret-value"} {
		if !maskedFieldMatches(config.MaskToken(value), value) {
			t.Errorf("mask of %q should match itself", value)
		}
	}
	if maskedFieldMatches("***zzzz", "different-value") {
		t.Error("an arbitrary masked string must not match an unrelated value")
	}
}
