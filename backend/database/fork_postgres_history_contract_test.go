package database

import (
	"aura/logging"
	"context"
	"testing"
)

type historyContractDB interface {
	InsertDownloadHistoryEntry(context.Context, DownloadHistoryEntry) logging.LogErrorInfo
	GetDownloadHistory(context.Context, int, int) ([]DownloadHistoryEntry, int, logging.LogErrorInfo)
	GetDownloadHistoryEntry(context.Context, int64) (DownloadHistoryEntry, bool, logging.LogErrorInfo)
	MarkDownloadHistoryRetried(context.Context, int64) logging.LogErrorInfo
	StopDownloadRetries(context.Context, int64) logging.LogErrorInfo
	DeleteDownloadHistoryForSet(context.Context, string, string, string, string) logging.LogErrorInfo
}

// TestHistoryStorageContract guards retry identity, backoff and original-source preservation.
func TestHistoryStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var db historyContractDB
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
			entry := DownloadHistoryEntry{TMDB_ID: "1", LibraryTitle: "Movies", SetID: "set", SetTitle: "Fixture", ImagesFailed: 1, Source: "download_window"}
			if info := db.InsertDownloadHistoryEntry(ctx, entry); info.Message != "" {
				t.Fatal(info)
			}
			entries, total, info := db.GetDownloadHistory(ctx, 10, 0)
			if info.Message != "" || total != 1 || len(entries) != 1 || entries[0].NextRetryAt == nil || entries[0].FailedImages == nil || entries[0].Status != "error" {
				t.Fatalf("history: %+v %d %+v", entries, total, info)
			}
			id := entries[0].ID
			if info := db.MarkDownloadHistoryRetried(ctx, id); info.Message != "" {
				t.Fatal(info)
			}
			entry.Source = "retry"
			if info := db.InsertDownloadHistoryEntry(ctx, entry); info.Message != "" {
				t.Fatal(info)
			}
			retried, found, info := db.GetDownloadHistoryEntry(ctx, id)
			if info.Message != "" || !found || retried.RetryCount != 1 || retried.Source != "download_window" || retried.NextRetryAt == nil || retried.NextRetryAt.Sub(retried.CreatedAt) < DownloadRetryDelays[1] {
				t.Fatalf("retry: %+v %+v", retried, info)
			}
			if info := db.StopDownloadRetries(ctx, id); info.Message != "" {
				t.Fatal(info)
			}
			retried, _, info = db.GetDownloadHistoryEntry(ctx, id)
			if info.Message != "" || retried.NextRetryAt != nil {
				t.Fatalf("retry not stopped: %+v %+v", retried, info)
			}
			if info := db.DeleteDownloadHistoryForSet(ctx, "1", "Movies", "", "set"); info.Message != "" {
				t.Fatal(info)
			}
			if _, found, info := db.GetDownloadHistoryEntry(ctx, id); info.Message != "" || found {
				t.Fatalf("history not deleted: %v %+v", found, info)
			}
		})
	}
}
