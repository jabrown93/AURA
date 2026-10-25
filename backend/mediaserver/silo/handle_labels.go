package silo

import (
	"aura/logging"
	"aura/models"
	"context"
)

func (s *Silo) AddLabelToMediaItem(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	// Silo does not support labels, so this function will simply return without doing anything
	return logging.LogErrorInfo{}
}
