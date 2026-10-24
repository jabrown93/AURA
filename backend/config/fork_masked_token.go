package config

// Fork: SavedMediaServerTokenForURL restores a masked media-server token only when the
// request still targets the URL that token was saved with. Restoring it for any URL would
// let a caller send the real token to a host of their choosing.
func SavedMediaServerTokenForURL(serverID, url string) string {
	if serverID == "" {
		if Current.MediaServer.URL != url {
			return ""
		}
		return Current.MediaServer.ApiToken
	}
	for _, server := range Current.MediaServers {
		if server.ID == serverID {
			if server.URL != url {
				return ""
			}
			return server.ApiToken
		}
	}
	return ""
}
