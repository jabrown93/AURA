package mediaserver

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"sync/atomic"
	"testing"
)

func TestApplyImagesWithKometaPreload(t *testing.T) {
	oldConfig, oldHook := config.Current, BeforeApplyImage
	t.Cleanup(func() { config.Current, BeforeApplyImage = oldConfig, oldHook })
	config.Current.MediaServer.Type = "Plex"
	config.Current.Images.Kometa.Enabled = false

	var mainCalls atomic.Int32
	BeforeApplyImage = func(context.Context, models.MediaItem, models.ImageFile) logging.LogErrorInfo {
		mainCalls.Add(1)
		return logging.LogErrorInfo{Message: "main server sentinel"}
	}
	season, episode := 2, 1
	images := []models.ImageFile{
		{Type: "poster"},
		{Type: "season_poster", SeasonNumber: &season},
		{Type: "titlecard", SeasonNumber: &season, EpisodeNumber: &episode},
	}
	selected := models.SelectedTypes{Poster: true, SeasonPoster: true, Titlecard: true}
	var finished int
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	results, _ := ApplyImagesWithKometaPreload(ctx, context.Background(), models.MediaItem{}, images, selected, func(update ImageUpdate) {
		if update.Finished {
			finished++
		}
	})
	if len(results) != len(images) || finished != len(images) {
		t.Fatalf("results = %d, finished = %d; want %d each", len(results), finished, len(images))
	}
	if mainCalls.Load() != 1 || results[0].Err.Message != "main server sentinel" {
		t.Fatalf("selected poster must reach main server: calls = %d, result = %+v", mainCalls.Load(), results[0])
	}
	for _, result := range results[1:] {
		if result.Err.Message != "" {
			t.Errorf("missing target reached main server: %+v", result)
		}
	}
}
