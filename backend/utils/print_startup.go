package utils

import (
	"aura/config"
	"aura/logging"
)

// Print application startup details
func PrintAppStartUpDetails(APP_VERSION, AUTHOR, LICENSE string, APP_PORT int, APP_NAME string) {
	logging.LOGGER.Info().
		Timestamp().
		Str("version", APP_VERSION).
		Str("build", config.AppBuildNumber).
		Str("commit", config.AppCommit).
		Str("build_date", config.AppBuildDate).
		Str("author", AUTHOR).
		Str("license", LICENSE).
		Int("port", APP_PORT).
		Msgf("Started %s", APP_NAME)
	config.AppVersion = APP_VERSION
	config.AppAuthor = AUTHOR
	config.AppLicense = LICENSE
	config.AppPort = APP_PORT
	config.AppName = APP_NAME
}
