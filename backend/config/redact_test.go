package config

import (
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRedactConfigYAML(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Auth.Password = "password-hash"
	cfg.Auth.APIKeyHash = "$argon2id$hash"
	cfg.Auth.OIDC = Config_Auth_OIDC{IssuerURL: "https://id.example.com", ClientID: "aura", ClientSecret: "client-secret",
		AllowedEmails: []string{"me@example.com"}, AllowedDomains: []string{"example.com"}}
	cfg.Mediux.ApiToken = "mediux-token"
	cfg.MediaServers = []Config_MediaServer{
		{ID: "1", Type: "Plex", URL: "http://10.1.1.100:32400", ApiToken: "plex-token", UserID: "user-id"},
		{ID: "2", Type: "Jellyfin", URL: "http://localhost:8096", ApiToken: ""},
	}
	cfg.SonarrRadarr.Applications = []Config_SonarrRadarrApp{{Type: "sonarr", Library: "Series", URL: "http://10.1.1.100:8989", ApiToken: "sonarr-token"}}
	cfg.Notifications.Providers = []Config_Notification_Provider{
		{Provider: "Discord", Enabled: true, Discord: &Config_Notification_Discord{Webhook: "https://discord.com/api/webhooks/1/secret"}},
		{Provider: "Gotify", Enabled: true, Gotify: &Config_Notification_Gotify{URL: "https://gotify.home.lan/message?token=abc", ApiToken: "gotify-token"}},
		{Provider: "Webhook", Enabled: true, Webhook: &Config_Notification_Webhook{URL: "https://hooks.home.lan/aura", Headers: map[string]string{"Authorization": "Bearer abc"}}},
	}
	cfg.Images.SaveImagesLocally.Path = "/data/images"

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	redactedData, err := RedactConfigYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"password-hash", "argon2id", "client-secret", "me@example.com", "example.com", "mediux-token", "plex-token",
		"user-id", "sonarr-token", "10.1.1.100", "discord.com", "gotify-token", "token=abc", "home.lan", "Bearer"} {
		if strings.Contains(string(redactedData), secret) {
			t.Errorf("redacted config still has %q", secret)
		}
	}

	var got Config
	if err := yaml.Unmarshal(redactedData, &got); err != nil {
		t.Fatalf("redacted config isn't valid YAML: %v", err)
	}
	// The same address gets the same name, loopback addresses and empty values are kept
	plexHost := strings.TrimSuffix(strings.TrimPrefix(got.MediaServers[0].URL, "http://"), ":32400")
	if !strings.HasPrefix(plexHost, "host-") || got.SonarrRadarr.Applications[0].URL != "http://"+plexHost+":8989" {
		t.Errorf("addresses = %q %q, want the same host name", got.MediaServers[0].URL, got.SonarrRadarr.Applications[0].URL)
	}
	if got.MediaServers[1].URL != "http://localhost:8096" || got.MediaServers[1].ApiToken != "" {
		t.Errorf("second server = %+v, want localhost and the empty token kept", got.MediaServers[1])
	}
	if got.MediaServers[0].ApiToken != RedactedValue || got.Auth.OIDC.ClientSecret != RedactedValue || got.Notifications.Providers[0].Discord.Webhook != RedactedValue {
		t.Error("secrets should be replaced with " + RedactedValue)
	}
	if got.Notifications.Providers[2].Webhook.Headers["Authorization"] != RedactedValue {
		t.Errorf("webhook headers = %v", got.Notifications.Providers[2].Webhook.Headers)
	}
	// Everything else is kept
	if got.MediaServers[0].Type != "Plex" || got.SonarrRadarr.Applications[0].Library != "Series" || got.Images.SaveImagesLocally.Path != "/data/images" ||
		got.Logging.Level != cfg.Logging.Level || got.Backups.Keep != cfg.Backups.Keep || !got.Notifications.Providers[0].Enabled {
		t.Errorf("redacted config lost settings: %+v", got)
	}
}

func TestRedactorText(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MediaServers = []Config_MediaServer{
		{ID: "1", Type: "Jellyfin", URL: "https://jellyfin.home.lan", ApiToken: "jellyfin-token"},
		{ID: "2", Type: "Plex", URL: "http://10.1.1.100:32400", ApiToken: "abc"},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRedactor()
	redactedConfig, err := r.ConfigYAML(data)
	if err != nil {
		t.Fatal(err)
	}

	line := `{"msg":"GET https://jellyfin.home.lan/Items?api_key=x failed","host":"jellyfin.home.lan","token":"jellyfin-token",` +
		`"plex":"10.1.1.100:32400","user":"me@example.com","mediux":"https://api.mediux.io/graphql","short":"abc"}`
	got := r.Text(line)
	for _, secret := range []string{"home.lan", "jellyfin-token", "10.1.1.100", "me@example.com", "api_key"} {
		if strings.Contains(got, secret) {
			t.Errorf("redacted text still has %q: %s", secret, got)
		}
	}
	// The addresses get the same names as in the config, public services and short values are kept
	jellyfinHost := strings.TrimPrefix(regexpFind(t, `https://host-\d+`, string(redactedConfig)), "https://")
	if !strings.Contains(got, `"host":"`+jellyfinHost+`"`) || !strings.Contains(got, "https://"+jellyfinHost+"/Items?redacted") {
		t.Errorf("jellyfin should be %s in the text: %s", jellyfinHost, got)
	}
	if !strings.Contains(got, "https://api.mediux.io/graphql") || !strings.Contains(got, `"short":"abc"`) {
		t.Errorf("public services and short values should be kept: %s", got)
	}
}

func regexpFind(t *testing.T, pattern, text string) string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindString(text)
	if match == "" {
		t.Fatalf("%s not found in %s", pattern, text)
	}
	return match
}
