package database

import (
	"aura/models"
	"testing"
)

// Fork: with overlapping library mappings (A -> B, B -> C), a destination must never become a
// later move's source, or A's saved sets would be carried on to C.
func TestMoveMediaItemsChainedMappings(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	seedMediaItem(t, db.conn, "1", "A", "", "100", "movie", 1)
	seedMediaItem(t, db.conn, "1", "B", "", "200", "movie", 2)

	moves := []MediaItemMove{
		{TMDB_ID: "1", OldLibrary: "A", NewItem: models.MediaItem{LibraryTitle: "B", RatingKey: "b-1", Type: "movie", Title: "Title", Year: 2024}, Found: true},
		{TMDB_ID: "1", OldLibrary: "B", NewItem: models.MediaItem{LibraryTitle: "C", RatingKey: "c-1", Type: "movie", Title: "Title", Year: 2024}, Found: true},
	}
	if Err := db.MoveMediaItems(ctx, moves); Err.Message != "" {
		t.Fatalf("MoveMediaItems: %s", Err.Message)
	}

	for library, set := range map[string]int{"B": 1, "C": 2} {
		if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE tmdb_id = '1' AND library_title = ? AND poster_set_id = ?;`, library, set); n != 1 {
			t.Errorf("library %s: set %d count = %d, want 1", library, set, n)
		}
		if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE tmdb_id = '1' AND library_title = ?;`, library); n != 1 {
			t.Errorf("library %s has %d saved sets, want 1", library, n)
		}
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM MediaItems WHERE library_title NOT IN ('B', 'C');`); n != 0 {
		t.Errorf("%d media items left outside B and C", n)
	}
}
