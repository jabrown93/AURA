package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

// TestPostgresSQLiteImport checks complete-table copy, secrets, queue recovery, IDs, and atomic refusal.
func TestPostgresSQLiteImport(t *testing.T) {
	ctx := context.Background()
	source := openTestMediaDB(t)
	for _, step := range []func(context.Context) logging.LogErrorInfo{source.CreateConfigTables, source.CreateAuthTable, source.CreateVersionTable} {
		if info := step(ctx); info.Message != "" {
			t.Fatal(info)
		}
	}
	if info := source.UpdateVersionTable(ctx, LATEST_DB_VERSION); info.Message != "" {
		t.Fatal(info)
	}
	secret, info := source.GetAuthTokenSecret(ctx)
	if info.Message != "" {
		t.Fatal(info)
	}
	if info = source.SaveConfigValues(ctx, map[string]string{"Authentication.APIKey": "keep-private"}, false); info.Message != "" {
		t.Fatal(info)
	}
	date := time.Date(2026, 10, 1, 12, 0, 0, 0, time.FixedZone("fixture", 7200))
	item := models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "100", Type: "movie", Title: "Import", Year: 2026, Movie: &models.MediaItemMovie{}}, PosterSets: []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set-1", Title: "Set", Type: "movie", DateCreated: date}, Images: []models.ImageFile{{ID: "image-1", Type: "poster", Modified: date}}}, LastDownloaded: date, SelectedTypes: models.SelectedTypes{Poster: true}, ForcePreloadMissing: true}}}
	if info = source.UpsertSavedItem(ctx, item); info.Message != "" {
		t.Fatal(info)
	}
	for _, query := range []string{
		`UPDATE MediaItems SET id=1000`,
		`UPDATE Movies SET media_item_id=1000`,
		`INSERT INTO IgnoredItems(tmdb_id,library_title,edition,mode,current_sets) VALUES('2','Movies','','until-new-set-available','a,b')`,
		`INSERT INTO DownloadQueueJobs(id,tmdb_id,library_title,edition,media_item_title,payload,status,worker_id) VALUES(100,'1','Movies','','Import','{}','processing','old-worker')`,
	} {
		if _, err := source.conn.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	pg := newPostgresTestDB(t)
	if _, info := pg.Init(ctx); info.Message != "" {
		t.Fatal(info)
	}
	report, err := pg.ImportSQLite(ctx, source.conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tables) != 18 || report.ResetQueueJobs != 1 {
		t.Fatalf("report: %+v", report)
	}
	imported, info := pg.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if info.Message != "" || imported.Total != 1 || !imported.Items[0].PosterSets[0].LastDownloaded.Equal(date) || !imported.Items[0].PosterSets[0].ForcePreloadMissing {
		t.Fatalf("saved import: %+v %+v", imported, info)
	}
	gotSecret, info := pg.GetAuthTokenSecret(ctx)
	if info.Message != "" || gotSecret != secret {
		t.Fatal("auth secret changed")
	}
	values, info := pg.GetAllConfigValues(ctx)
	if info.Message != "" || values["Authentication.APIKey"] != "keep-private" {
		t.Fatal("configuration changed during import")
	}
	var id int64
	if err = pg.conn.QueryRowContext(ctx, `INSERT INTO MediaItems(tmdb_id,library_title,rating_key,type,title,year) VALUES('3','Movies','300','movie','Next',2026) RETURNING id`).Scan(&id); err != nil || id != 1001 {
		t.Fatalf("sequence: %d %v", id, err)
	}
	var status, worker, sets string
	if err = pg.conn.QueryRowContext(ctx, `SELECT status,worker_id FROM DownloadQueueJobs WHERE id=100`).Scan(&status, &worker); err != nil || status != "pending" || worker != "" {
		t.Fatalf("interrupted job: %q %q %v", status, worker, err)
	}
	if err = pg.conn.QueryRowContext(ctx, `SELECT current_sets::text FROM IgnoredItems`).Scan(&sets); err != nil || sets != `["a", "b"]` {
		t.Fatalf("ignored sets: %q %v", sets, err)
	}
	if _, err = pg.ImportSQLite(ctx, source.conn); err == nil {
		t.Fatal("accepted nonempty target")
	}

	t.Run("unknown-column", func(t *testing.T) {
		target := newPostgresTestDB(t)
		if _, info := target.Init(ctx); info.Message != "" {
			t.Fatal(info)
		}
		if _, err := source.conn.ExecContext(ctx, `ALTER TABLE Config ADD COLUMN future TEXT`); err != nil {
			t.Fatal(err)
		}
		if _, err := target.ImportSQLite(ctx, source.conn); err == nil {
			t.Fatal("silently discarded unknown column")
		}
		if _, err := source.conn.ExecContext(ctx, `ALTER TABLE Config DROP COLUMN future`); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := target.conn.QueryRowContext(ctx, `SELECT count(*) FROM AUTH`).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial auth import after schema refusal")
		}
	})

	t.Run("rollback", func(t *testing.T) {
		target := newPostgresTestDB(t)
		if _, info := target.Init(ctx); info.Message != "" {
			t.Fatal(info)
		}
		if _, err := source.conn.ExecContext(ctx, `INSERT INTO StatisticsSnapshots(snapshot_date,data) VALUES('2026-10-01','not-json')`); err != nil {
			t.Fatal(err)
		}
		if _, err := target.ImportSQLite(ctx, source.conn); err == nil {
			t.Fatal("accepted invalid JSON")
		}
		for _, table := range []string{"AUTH", "Config", "MediaItems", "SavedItems", "DownloadQueueJobs"} {
			var count int
			if err := target.conn.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rollback %s: %d %v", table, count, err)
			}
		}
	})
}
