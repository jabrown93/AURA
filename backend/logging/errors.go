package logging

func Error_DBClientNotInitialized() LogErrorInfo {
	return LogErrorInfo{
		Message: "Database client is not initialized",
		Help:    "Restart application to try again.",
	}
}

func Error_BaseUrlParsing(err error) (message, help string, detail map[string]any) {
	message = "Failed to parse base URL"
	help = "Ensure the URL is valid"
	detail = map[string]any{"error": err.Error()}
	return message, help, detail
}
