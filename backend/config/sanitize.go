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
	//c.Auth.Password = MaskToken(c.Auth.Password)
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
