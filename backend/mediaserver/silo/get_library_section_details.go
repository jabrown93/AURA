package silo

import (
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
)

func (s *Silo) GetLibrarySectionDetails(ctx context.Context, library *models.LibrarySection) (found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Details for Library Section: %s", library.Title,
	), logging.LevelDebug)
	defer logAction.Complete()

	found = false

	// Get all Library Sections from Silo
	sections, Err := s.GetLibrarySections(ctx, s.Config)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return found, *logAction.Error
	}

	// Find the specific library section and update its details
	for _, section := range sections {
		if section.Title == library.Title {
			library.ID = section.ID
			library.Type = section.Type
			library.Paths = section.Paths
			found = true
			break
		}
	}
	if !found {
		logAction.SetError("Library section not found", fmt.Sprintf("Ensure the library section '%s' exists on the Silo server.", library.Title), map[string]any{
			"LibrarySectionTitle": library.Title,
		})
		return found, *logAction.Error
	}

	return found, logging.LogErrorInfo{}
}
