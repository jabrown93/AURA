package config

import (
	"context"
	"strings"
	"testing"

	"aura/logging"
)

func TestSanitizeConfigMasksSecrets(t *testing.T) {
	cfg := testConfig()
	cfg.Auth.Password = "$argon2id$v=19$m=65536,t=1,p=4$password-hash"
	cfg.Mediux.ApiToken = "mediux-token"

	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	sanitized := cfg.SanitizeConfig(ctx)

	secrets := map[string]string{
		"Auth.Password":            sanitized.Auth.Password,
		"Auth.OIDC.ClientSecret":   sanitized.Auth.OIDC.ClientSecret,
		"MediaServer.ApiToken":     sanitized.MediaServer.ApiToken,
		"MediaServers[0].ApiToken": sanitized.MediaServers[0].ApiToken,
		"MediaServers[1].ApiToken": sanitized.MediaServers[1].ApiToken,
		"Mediux.ApiToken":          sanitized.Mediux.ApiToken,
		"TMDB.ApiToken":            sanitized.TMDB.ApiToken,
		"SonarrRadarr.ApiToken":    sanitized.SonarrRadarr.Applications[0].ApiToken,
		"Webhook.Headers":          sanitized.Notifications.Providers[1].Webhook.Headers["Authorization"],
	}
	for name, value := range secrets {
		if value != "" && !strings.HasPrefix(value, "***") {
			t.Errorf("%s is not masked: %q", name, value)
		}
	}
	// Sanitizing must not change the original config
	if cfg.Notifications.Providers[1].Webhook.Headers["Authorization"] != "Bearer 1" || !strings.HasPrefix(cfg.Auth.Password, "$argon2id") ||
		cfg.MediaServers[1].ApiToken != "67890" {
		t.Error("SanitizeConfig changed the original config")
	}
}
