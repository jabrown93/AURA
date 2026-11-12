package database

import (
	"aura/models"
	"testing"
)

// TestGetAllSavedSetsImages guards against SQLite encoding nested image arrays as strings.
func TestGetAllSavedSetsImages(t *testing.T) {
	for _, withImage := range []bool{false, true} {
		name := "empty"
		if withImage {
			name = "populated"
		}
		t.Run(name, func(t *testing.T) {
			db := openTestMediaDB(t)
			seedMediaItem(t, db.conn, "1", "Movies", "", "100", "movie", 1)
			exec(t, db.conn, `UPDATE SavedItems SET last_downloaded = '2026-10-01 00:00:00+00:00';`)
			if withImage {
				exec(t, db.conn, `INSERT INTO ImageFiles (poster_set_id, item_tmdb_id, image_id, image_type, image_last_updated)
VALUES (1, '1', 'poster-1', 'poster', '2026-10-01 00:00:00+00:00');`)
			}

			out, Err := db.GetAllSavedSets(testContext(), models.DBFilter{ItemsPerPage: -1})
			if Err.Message != "" {
				t.Fatalf("%s: %+v", Err.Message, Err.Detail)
			}
			if len(out.Items) != 1 || len(out.Items[0].PosterSets) != 1 {
				t.Fatalf("expected one item with one set, got %+v", out.Items)
			}
			images := out.Items[0].PosterSets[0].Images
			if withImage {
				if len(images) != 1 || images[0].ID != "poster-1" || images[0].Type != "poster" {
					t.Fatalf("unexpected images: %+v", images)
				}
			} else if images == nil || len(images) != 0 {
				t.Fatalf("expected empty image array, got %#v", images)
			}
		})
	}
}
