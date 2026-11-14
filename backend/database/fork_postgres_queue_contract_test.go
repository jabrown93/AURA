package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"sync"
	"testing"
)

type queueContractDB interface {
	EnqueueDownloadQueueJob(context.Context, models.DBSavedItem) (int64, logging.LogErrorInfo)
	ClaimNextDownloadQueueJob(context.Context, string) (DownloadQueueJob, bool, logging.LogErrorInfo)
	FinishDownloadQueueJob(context.Context, int64, string, string, []string, []string) logging.LogErrorInfo
	ResetInterruptedDownloadQueueJobs(context.Context, string) (int64, logging.LogErrorInfo)
	GetAllDownloadQueueJobs(context.Context) ([]DownloadQueueJob, logging.LogErrorInfo)
}

// TestQueueStorageContract checks concurrent claims, same-item exclusion and restart recovery.
func TestQueueStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db queueContractDB
			if engine == "sqlite" {
				db = openTestMediaDB(t)
			} else {
				pg := newPostgresTestDB(t)
				if _, info := pg.Init(context.Background()); info.Message != "" {
					t.Fatal(info)
				}
				db = pg
			}
			ctx := context.Background()
			for _, id := range []string{"1", "1", "2"} {
				item := models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: id, LibraryTitle: "Movies", Type: "movie", Title: "Queued"}}
				if _, info := db.EnqueueDownloadQueueJob(ctx, item); info.Message != "" {
					t.Fatal(info)
				}
			}
			var wg sync.WaitGroup
			jobs := make(chan DownloadQueueJob, 3)
			for range 3 {
				wg.Go(func() {
					job, found, info := db.ClaimNextDownloadQueueJob(ctx, "worker-a")
					if info.Message != "" {
						t.Error(info)
					} else if found {
						jobs <- job
					}
				})
			}
			wg.Wait()
			close(jobs)
			seen := map[string]bool{}
			for job := range jobs {
				if seen[job.TMDB_ID] || job.StartedAt == nil || job.Payload.MediaItem.TMDB_ID != job.TMDB_ID {
					t.Fatalf("duplicate/invalid claim: %+v", job)
				}
				seen[job.TMDB_ID] = true
			}
			if len(seen) != 2 {
				t.Fatalf("expected one claim per distinct item, got %v", seen)
			}
			if count, info := db.ResetInterruptedDownloadQueueJobs(ctx, "worker-b"); info.Message != "" || count != 2 {
				t.Fatalf("restart reset: %d %+v", count, info)
			}
			for range 3 {
				job, found, info := db.ClaimNextDownloadQueueJob(ctx, "worker-b")
				if info.Message != "" || !found {
					t.Fatalf("reclaim: %v %+v", found, info)
				}
				if info := db.FinishDownloadQueueJob(ctx, job.ID, "success", "done", nil, nil); info.Message != "" {
					t.Fatal(info)
				}
			}
			all, info := db.GetAllDownloadQueueJobs(ctx)
			if info.Message != "" || len(all) != 3 {
				t.Fatalf("queue read: %+v %+v", all, info)
			}
			for _, job := range all {
				if job.Status != "success" || job.FinishedAt == nil || job.ResultErrors == nil || job.ResultWarnings == nil {
					t.Fatalf("lost terminal result: %+v", job)
				}
			}
		})
	}
}
