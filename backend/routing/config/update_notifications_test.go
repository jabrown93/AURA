package routes_config

import (
	"aura/config"
	"testing"
)

const webhookSecret = "Bearer super-secret-value"

func webhookNotifications(url string, headers map[string]string) config.Config_Notifications {
	return config.Config_Notifications{
		Enabled: true,
		Providers: []config.Config_Notification_Provider{
			{
				Provider: "Webhook",
				Enabled:  true,
				Webhook:  &config.Config_Notification_Webhook{URL: url, Headers: headers},
			},
		},
	}
}

// Re-sending the masked header the UI was given means "leave it alone". Taking it at face
// value would overwrite the real credential with its own mask.
func TestWebhookMaskedHeaderIsRestored(t *testing.T) {
	oldNotifications := webhookNotifications("https://hooks.example.com/notify", map[string]string{
		"Authorization": webhookSecret,
	})
	newNotifications := webhookNotifications("https://hooks.example.com/notify", map[string]string{
		"Authorization": config.MaskToken(webhookSecret),
	})

	_, valid := checkConfigDifferences_Notifications(testContext(), oldNotifications, &newNotifications)

	if got := newNotifications.Providers[0].Webhook.Headers["Authorization"]; got != webhookSecret {
		t.Errorf("header = %q, want the stored credential preserved", got)
	}
	if !valid {
		t.Error("restoring a masked header must leave the config valid")
	}
}

// The credential was issued for one endpoint. Restoring it alongside a new URL would send
// the real Authorization value to whatever endpoint the caller supplied.
func TestWebhookMaskedHeaderRejectedWhenURLChanges(t *testing.T) {
	oldNotifications := webhookNotifications("https://hooks.example.com/notify", map[string]string{
		"Authorization": webhookSecret,
	})
	newNotifications := webhookNotifications("https://attacker.example.net/collect", map[string]string{
		"Authorization": config.MaskToken(webhookSecret),
	})

	_, valid := checkConfigDifferences_Notifications(testContext(), oldNotifications, &newNotifications)

	if valid {
		t.Fatal("a masked header must not be restored across a URL change")
	}
	if got := newNotifications.Providers[0].Webhook.Headers["Authorization"]; got == webhookSecret {
		t.Errorf("header = %q, want the real credential withheld from the new URL", got)
	}
}

// A renamed key has no stored counterpart, so the mask cannot be resolved. Saving it would
// write the literal mask text as the credential and break webhook auth.
func TestWebhookMaskedHeaderRejectedWhenKeyRenamed(t *testing.T) {
	oldNotifications := webhookNotifications("https://hooks.example.com/notify", map[string]string{
		"Authorization": webhookSecret,
	})
	newNotifications := webhookNotifications("https://hooks.example.com/notify", map[string]string{
		"X-Auth": config.MaskToken(webhookSecret),
	})

	_, valid := checkConfigDifferences_Notifications(testContext(), oldNotifications, &newNotifications)

	if valid {
		t.Fatal("a masked header under a renamed key must be rejected, not saved as the mask")
	}
}
