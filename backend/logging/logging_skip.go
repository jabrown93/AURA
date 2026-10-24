package logging

import (
	"net/http"
	"regexp"
)

func ShouldSkipLogging(request *http.Request, ld *LogData) bool {
	// Define patterns for paths to skip logging
	skipPatterns := []string{
		`\.js$`,
		`\.css$`,
		`manifest\.json$`,
		`\.woff2$`,
		`\.woff$`,
		`\.ttf$`,
		`\.eot$`,
		`\.otf$`,
		`\.ico$`,
		`\.png$`,
		`\.jpg$`,
		`\.jpeg$`,
		`\.gif$`,
		`\.webp$`,
		`\.avif$`,
		`\.bmp$`,
		`\.tiff$`,
		`\.svg$`,
		`logo.*$`,
		`^/assets/.*$`,
		`^/favicon.ico$`,
		// AURA API Routes to skip
		`^/api/images/.*$`,
		`^/api/health$`,
		`^/api/config$`,
		`^/api/download/queue$`,
	}

	if ld != nil {
		if ld.Status == StatusError {
			return false
		}
	}

	// Check if the path matches any skip pattern
	for _, pattern := range skipPatterns {
		matched, err := regexp.MatchString(pattern, request.URL.Path)
		if err == nil && matched {
			return true
		}
	}

	return request.Method == "OPTIONS"
}
