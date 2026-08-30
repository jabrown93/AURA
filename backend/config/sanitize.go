package config

import (
	"aura/logging"
	"context"
)

func (config *Config) SanitizeConfig(ctx context.Context) *Config {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Sanitizing Configuration", logging.LevelTrace)
	defer logAction.Complete()

	if config == nil {
		return &Config{}
	}

	// Create a deep copy of the config to avoid modifying the original
	c := *config

	// Mask top-level sensitive fields (ensure these are value fields, not shared pointers).
	// The password hash is logged on startup through print.go and returned by /api/config;
	// update.go restores the stored value when the mask is sent back unchanged.
	c.Auth.Password = MaskToken(c.Auth.Password)
	c.Auth.OIDC.ClientSecret = MaskToken(c.Auth.OIDC.ClientSecret)
	c.Mediux.ApiToken = MaskToken(c.Mediux.ApiToken)
	c.TMDB.ApiToken = MaskToken(c.TMDB.ApiToken)
	c.MediaServer.ApiToken = MaskToken(c.MediaServer.ApiToken)

	// Deep copy notifications.providers slice and nested pointer
	if len(config.Notifications.Providers) > 0 {
		c.Notifications.Providers = make([]Config_Notification_Provider, len(config.Notifications.Providers))
		for i, p := range config.Notifications.Providers {
			cp := p // copy struct
			if p.Discord != nil {
				cp.Discord = &Config_Notification_Discord{
					Webhook: MaskWebhookURL(p.Discord.Webhook),
				}
			}
			if p.Pushover != nil {
				cp.Pushover = &Config_Notification_Pushover{
					ApiToken: MaskToken(p.Pushover.ApiToken),
					UserKey:  MaskToken(p.Pushover.UserKey),
				}
			}
			if p.Gotify != nil {
				cp.Gotify = &Config_Notification_Gotify{
					URL:      p.Gotify.URL, // URL is not sensitive
					ApiToken: MaskToken(p.Gotify.ApiToken),
				}
			}
			// Without this branch cp.Webhook stays the original pointer, so the headers -
			// which carry the Authorization value sent to the webhook - would be returned
			// by /api/config and logged by print.go in full. URL is left alone to match
			// Gotify: it is the user's own endpoint, and masking it would need a matching
			// unmask in update.go before the config could be saved back.
			if p.Webhook != nil {
				maskedHeaders := make(map[string]string, len(p.Webhook.Headers))
				for key, value := range p.Webhook.Headers {
					maskedHeaders[key] = MaskToken(value)
				}
				cp.Webhook = &Config_Notification_Webhook{
					URL:     p.Webhook.URL,
					Headers: maskedHeaders,
				}
			}
			c.Notifications.Providers[i] = cp
		}
	}

	// Deep copy SonarrRadarr slice
	if len(config.SonarrRadarr.Applications) > 0 {
		c.SonarrRadarr.Applications = make([]Config_SonarrRadarrApp, len(config.SonarrRadarr.Applications))
		for i, app := range config.SonarrRadarr.Applications {
			c.SonarrRadarr.Applications[i] = Config_SonarrRadarrApp{
				Type:     app.Type,
				Library:  app.Library,
				URL:      app.URL,
				ApiToken: MaskToken(app.ApiToken),
			}
		}
	}

	return &c
}
