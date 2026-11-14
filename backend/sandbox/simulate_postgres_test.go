package sandbox

import (
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresSimulationSQL(t *testing.T) {
	query := `UPDATE Episodes SET path = path || '.sandbox' WHERE episode_number = ? AND season_id = ` + seasonRowSQL
	want := `UPDATE Episodes SET path = path || '.sandbox' WHERE episode_number = $1 AND season_id = (SELECT s.id FROM Seasons s JOIN Series sr ON sr.id = s.series_id WHERE sr.media_item_id = (SELECT id FROM MediaItems WHERE tmdb_id = $2 AND library_title = $3 AND edition = $4) AND s.season_number = $5)`
	if got := postgresSimulationSQL(query); got != want {
		t.Fatalf("query = %s, want %s", got, want)
	}
}

func TestSimulatePostgres(t *testing.T) {
	dsn := os.Getenv("AURA_TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("AURA_TEST_POSTGRES_DSN is required in CI")
		}
		t.Skip("AURA_TEST_POSTGRES_DSN is not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test DSN")
	}
	control := stdlib.OpenDB(*cfg)
	defer control.Close()
	schema := fmt.Sprintf("aura_sandbox_test_%d", time.Now().UnixNano())
	if _, err := control.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer control.Exec("DROP SCHEMA " + schema + " CASCADE")
	cfg.RuntimeParams["search_path"] = schema
	t.Setenv("AURA_DB_DRIVER", "postgres")
	t.Setenv("AURA_POSTGRES_DSN", cfg.ConnString())
	oldClient := database.Client
	defer func() { database.Client = oldClient }()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "sandbox postgres test")
	defer ld.Log()
	if _, info := database.Init(ctx); info.Message != "" {
		t.Fatal(info.Message)
	}
	defer database.Close(ctx)
	conn, _, info := database.GetDBConnection(ctx)
	if info.Message != "" {
		t.Fatal(info.Message)
	}
	for _, change := range []string{ChangeImageUpdated, ChangeImageNew, ChangeMovieFileChanged, ChangeEpisodeFileChanged, ChangeEpisodeAdded, ChangeSeasonAdded} {
		t.Run(change, func(t *testing.T) {
			for _, query := range []string{
				`TRUNCATE MediaItems, PosterSets RESTART IDENTITY CASCADE`,
				`INSERT INTO MediaItems (tmdb_id,library_title,edition,rating_key,type,title,year) VALUES ('1','TV','4K','1','show','Test',2026)`,
				`INSERT INTO Movies (media_item_id,path,size,duration) VALUES (1,'movie',1,1)`,
				`INSERT INTO Series (media_item_id) VALUES (1)`,
				`INSERT INTO Seasons (series_id,rating_key,season_number) VALUES (1,'s1',1)`,
				`INSERT INTO Episodes (season_id,rating_key,episode_number,path,size,duration) VALUES (1,'e1',1,'episode',1,1)`,
				`INSERT INTO PosterSets (set_id,type,title,"user") VALUES ('set','show','Test','test')`,
				`INSERT INTO ImageFiles (poster_set_id,item_tmdb_id,image_id,image_type,image_last_updated) VALUES (1,'1','image','poster','2026-01-01')`,
			} {
				if _, err := conn.ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			info := Simulate(ctx, Change{TMDB_ID: "1", LibraryTitle: "TV", Edition: "4K", Change: change, ImageID: "image", ImageType: "poster", SeasonNumber: 1, EpisodeNumber: 1})
			if info.Message != "" {
				t.Fatal(info.Message)
			}
			checks := map[string]string{
				ChangeImageUpdated:       `SELECT count(*) FROM ImageFiles WHERE image_last_updated = '2020-01-01 00:00:00+00:00'`,
				ChangeImageNew:           `SELECT 1 - count(*) FROM ImageFiles`,
				ChangeMovieFileChanged:   `SELECT count(*) FROM Movies WHERE path = 'movie.sandbox' AND size = 1048577`,
				ChangeEpisodeFileChanged: `SELECT count(*) FROM Episodes WHERE path = 'episode.sandbox' AND size = 1048577`,
				ChangeEpisodeAdded:       `SELECT 1 - count(*) FROM Episodes`,
				ChangeSeasonAdded:        `SELECT 1 - (SELECT count(*) FROM Seasons) - (SELECT count(*) FROM Episodes)`,
			}
			var changed int
			if err := conn.QueryRowContext(ctx, checks[change]).Scan(&changed); err != nil || changed != 1 {
				t.Fatalf("simulation effect = %d, error %v", changed, err)
			}
		})
	}
}
