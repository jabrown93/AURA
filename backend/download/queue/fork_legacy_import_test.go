package downloadqueue

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportLegacyQueueFiles(t *testing.T) {
	ctx, _ := setupRetryStubs(t)
	dir := t.TempDir()
	oldPath := config.ConfigPath
	config.ConfigPath = dir
	var queued []models.DBSavedItem
	oldEnqueue := enqueueLegacyJob
	t.Cleanup(func() { config.ConfigPath = oldPath; enqueueLegacyJob = oldEnqueue })
	enqueueLegacyJob = func(_ context.Context, item models.DBSavedItem) (int64, logging.LogErrorInfo) {
		queued = append(queued, item)
		return int64(len(queued)), logging.LogErrorInfo{}
	}

	q := filepath.Join(dir, "download-queue")
	if err := os.MkdirAll(filepath.Join(q, "collections"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(q, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Movies_603_1.json", `{"media_item":{"tmdb_id":"603","library_title":"Movies"},"poster_sets":[{"id":"s1"}]}`)
	write("error_Movies_604_1.json", `{"media_item":{"tmdb_id":"604","library_title":"Movies"}}`)
	write("broken.json", `{`)

	ImportLegacyQueueFiles(ctx)

	if len(queued) != 1 || queued[0].MediaItem.TMDB_ID != "603" || len(queued[0].PosterSets) != 1 {
		t.Fatalf("queued = %+v, want the one pending job", queued)
	}
	if _, err := os.Stat(filepath.Join(q, "imported", "Movies_603_1.json")); err != nil {
		t.Error("imported file not moved aside")
	}
	for _, left := range []string{"error_Movies_604_1.json", "broken.json"} {
		if _, err := os.Stat(filepath.Join(q, left)); err != nil {
			t.Errorf("%s should stay in place", left)
		}
	}

	ImportLegacyQueueFiles(ctx)
	if len(queued) != 1 {
		t.Error("a second start imported again")
	}
}
