package database

import "testing"

func TestAutoIgnoreMediaItem(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	for _, mode := range []string{"", "always", "until-set-available", "until-new-set-available"} {
		t.Run("mode="+mode, func(t *testing.T) {
			exec(t, db.conn, `DELETE FROM IgnoredItems`)
			if mode != "" {
				exec(t, db.conn, `INSERT INTO IgnoredItems (tmdb_id,library_title,edition,mode,current_sets) VALUES ('1','Movies','Director',?,'set1')`, mode)
			}
			for range 2 {
				if err := db.AutoIgnoreMediaItem(ctx, "1", "Movies", "Director"); err.Message != "" {
					t.Fatal(err.Message)
				}
			}
			var got, sets string
			if err := db.conn.QueryRow(`SELECT mode,current_sets FROM IgnoredItems WHERE tmdb_id='1' AND library_title='Movies' AND edition='Director'`).Scan(&got, &sets); err != nil {
				t.Fatal(err)
			}
			want := mode
			if want == "" {
				want = "until-set-available"
			}
			if got != want || (mode != "" && sets != "set1") {
				t.Fatalf("got %q/%q, want mode %q with original sets preserved", got, sets, want)
			}
		})
	}
}
