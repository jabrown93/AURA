package config

import (
	"reflect"
	"testing"
)

func TestCloneIsIndependent(t *testing.T) {
	original := testConfig()
	original.MediaServer.UserID = "admin-user-id"
	clone := original.Clone()

	if !reflect.DeepEqual(original, clone) {
		t.Fatal("clone should equal the original")
	}

	// Changing the clone's lists, maps and pointers must not change the original
	clone.Auth.AllowedOrigins[0] = "changed"
	clone.Notifications.Providers[0].Discord.Webhook = "changed"
	clone.Notifications.Providers[1].Webhook.Headers["Authorization"] = "changed"
	*clone.Jobs.AutoDownload.Enabled = true
	clone.MediaServer.Libraries[0].Title = "changed"

	if original.Auth.AllowedOrigins[0] == "changed" ||
		original.Notifications.Providers[0].Discord.Webhook == "changed" ||
		original.Notifications.Providers[1].Webhook.Headers["Authorization"] == "changed" ||
		*original.Jobs.AutoDownload.Enabled ||
		original.MediaServer.Libraries[0].Title == "changed" {
		t.Error("changing the clone changed the original")
	}

	// nil lists stay nil
	if original.Auth.OIDC.AllowedEmails != nil || clone.Auth.OIDC.AllowedEmails != nil {
		t.Error("nil list should stay nil")
	}
}
