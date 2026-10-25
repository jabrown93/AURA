package silo

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
)

func (s *Silo) GetLibrarySections(ctx context.Context, msConfig config.Config_MediaServer) (sections []models.LibrarySection, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Silo: Fetching Library Sections", logging.LevelInfo)
	defer logAction.Complete()

	sections = []models.LibrarySection{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Silo server API request
	u, err := apiURL(msConfig.URL, "libraries")
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return sections, *logAction.Error
	}

	// Make the HTTP Request to Silo
	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return sections, *logAction.Error
	}

	// Decode the Response
	var siloResp SiloLibrariesResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Library Sections Response")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return sections, *logAction.Error
	}

	logAction.AppendResult("total_sections", len(siloResp.Items))

	// Map the Silo libraries to the LibrarySection model
	for _, siloLibrary := range siloResp.Items {
		sectionType := map[string]string{
			"movies": "movie",
			"series": "show",
			"mixed":  "mixed",
		}[siloLibrary.Type]
		if sectionType == "" {
			logAction.AppendWarning(fmt.Sprintf("skipped '%s'", siloLibrary.Name), map[string]string{"type": siloLibrary.Type})
			continue
		}
		// Disabled libraries are queued for deletion on the Silo server
		if !siloLibrary.Enabled {
			logAction.AppendWarning(fmt.Sprintf("skipped '%s'", siloLibrary.Name), map[string]string{"reason": "library is disabled"})
			continue
		}
		section := models.LibrarySection{}
		section.ID = siloLibrary.ID
		section.Title = siloLibrary.Name
		section.Type = sectionType
		section.Paths = siloLibrary.Paths
		sections = append(sections, section)
	}

	logAction.AppendResult("found", len(sections))
	return sections, logging.LogErrorInfo{}
}
