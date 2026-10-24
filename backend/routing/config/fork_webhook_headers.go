package routes_config

import (
	"aura/config"
	"strings"
)

// Fork: upstream restores a masked webhook header by key within one provider, which can save the
// mask itself as the header value, or send a real header to a changed URL.

// providerMapNotifications creates a map of notification providers keyed by provider name.
// restoreMaskedWebhookHeaders puts the stored value back wherever the UI echoed back a
// masked header, and reports whether every mask was resolvable.
//
// Providers are paired by their position in the webhook list, because that is the only
// stable identity available: URL, header key and mask can all coincide across two providers
// (MaskToken is derived from the value, so two secrets ending the same way mask the same),
// and searching by those would hand back another provider's credential. The URL must still
// match at that position, so a masked header is never re-pointed at a different endpoint -
// reordering or removing a provider simply fails the save and asks for the full value.
func restoreMaskedWebhookHeaders(oldProviders, newProviders []config.Config_Notification_Provider) bool {
	oldWebhooks := webhookProviders(oldProviders)
	newWebhooks := webhookProviders(newProviders)

	resolved := true
	for i, newWebhook := range newWebhooks {
		for key, value := range newWebhook.Headers {
			if !config.IsMaskedField(value) {
				continue
			}
			if i >= len(oldWebhooks) || oldWebhooks[i].URL != newWebhook.URL {
				resolved = false
				continue
			}
			// Requiring the mask this exact value would produce stops a caller passing an
			// arbitrary "***" string to fish out a stored credential.
			stored, ok := oldWebhooks[i].Headers[key]
			if !ok || value != config.MaskToken(stored) {
				resolved = false
				continue
			}
			newWebhook.Headers[key] = stored
		}
	}
	return resolved
}

// Provider names are matched case-insensitively because this runs before validation, and
// ValidateNotificationsProvider title-cases the name afterwards. An exact match would skip a
// provider submitted as "webhook", leaving its masked headers to be saved as the credential.
func webhookProviders(items []config.Config_Notification_Provider) []*config.Config_Notification_Webhook {
	var webhooks []*config.Config_Notification_Webhook
	for _, provider := range items {
		if strings.EqualFold(provider.Provider, "Webhook") && provider.Webhook != nil {
			webhooks = append(webhooks, provider.Webhook)
		}
	}
	return webhooks
}
