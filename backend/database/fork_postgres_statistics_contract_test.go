package database

import (
	"aura/models"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestStatisticsStorageContract exercises public statistics functions against both engines.
func TestStatisticsStorageContract(t *testing.T) {
	for _, engine := range []string{"sqlite", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			old := Client
			t.Cleanup(func() { Client = old })
			if engine == "sqlite" {
				Client = openTestMediaDB(t)
			} else {
				pg := newPostgresTestDB(t)
				if _, info := pg.Init(context.Background()); info.Message != "" {
					t.Fatal(info)
				}
				Client = pg
			}
			ctx := context.Background()
			date := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
			for i := range 2 {
				event := models.DownloadEvent{TMDB_ID: "1", LibraryTitle: "Movies", MediaItemType: "movie", SetID: "set", SetCreator: "Creator", Source: "manual", Kind: "download", ImagesSucceeded: 2, ImageTypes: map[string]int{"poster": 2}, Bytes: 42, CreatedAt: date.Add(time.Duration(i) * 24 * time.Hour)}
				if info := Client.InsertDownloadEvent(ctx, event); info.Message != "" {
					t.Fatal(info)
				}
			}
			totals, info := GetDownloadEventTotals(ctx, time.Time{})
			if info.Message != "" || len(totals) != 1 || totals[0].Events != 2 || totals[0].ImagesSucceeded != 4 || totals[0].Bytes != 84 {
				t.Fatalf("totals: %+v %+v", totals, info)
			}
			filtered, info := GetDownloadEventTotals(ctx, date.Add(time.Hour))
			if info.Message != "" || len(filtered) != 1 || filtered[0].Events != 1 {
				t.Fatalf("filtered totals: %+v %+v", filtered, info)
			}
			for _, period := range []string{"day", "week", "month"} {
				buckets, info := GetDownloadEventTimeline(ctx, time.Time{}, period)
				want := 1
				if period == "day" {
					want = 2
				}
				if info.Message != "" || len(buckets) != want {
					t.Fatalf("%s buckets: %+v %+v", period, buckets, info)
				}
			}
			for _, groupBy := range []string{"creator", "source"} {
				groups, info := GetDownloadEventGroups(ctx, date, groupBy, 10)
				if info.Message != "" || len(groups) != 1 || groups[0].Sets != 2 || groups[0].Images != 4 {
					t.Fatalf("groups: %+v %+v", groups, info)
				}
			}
			types, info := GetDownloadEventImageTypes(ctx, time.Time{})
			if info.Message != "" || len(types) != 1 || types[0].Name != "poster" || types[0].Count != 4 {
				t.Fatalf("image types: %+v %+v", types, info)
			}
			repeats, info := GetRedownloadCount(ctx, date.Add(time.Hour))
			if info.Message != "" || repeats != 1 {
				t.Fatalf("redownloads: %d %+v", repeats, info)
			}
			if info := Client.InsertDownloadHistoryEntry(ctx, DownloadHistoryEntry{TMDB_ID: "1", LibraryTitle: "Movies", FailedImages: []models.ImageDownloadResult{{FailureReason: "Bad image"}}}); info.Message != "" {
				t.Fatal(info)
			}
			reasons, info := GetFailureReasons(ctx, 10)
			if info.Message != "" || len(reasons) != 1 || reasons[0].Name != "Bad image" || reasons[0].Count != 1 {
				t.Fatalf("failures: %+v %+v", reasons, info)
			}
			count, info := CountDownloadHistory(ctx)
			if info.Message != "" || count != 1 {
				t.Fatalf("history count: %d %+v", count, info)
			}
			if a, b, info := CollectionCounts(ctx); info.Message != "" || a != 0 || b != 0 {
				t.Fatalf("collection counts: %d %d %+v", a, b, info)
			}
			for _, snapshot := range []string{`{"count":1}`, `{"count":2}`} {
				if info := UpsertStatisticsSnapshot(ctx, "2026-01-05", snapshot); info.Message != "" {
					t.Fatal(info)
				}
			}
			snapshots, info := GetStatisticsSnapshots(ctx, "2026-01-01")
			var snapshot map[string]int
			if info.Message != "" || len(snapshots) != 1 || json.Unmarshal([]byte(snapshots[0].Data), &snapshot) != nil || snapshot["count"] != 2 {
				t.Fatalf("snapshot: %+v %+v", snapshots, info)
			}
		})
	}
}
