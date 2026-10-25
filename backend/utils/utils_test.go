package utils

import (
	"aura/logging"
	"aura/models"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func num(n int) *int { return &n }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestConvertDateStringToTime(t *testing.T) {
	want := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	tests := map[string]string{
		"unix seconds":      "1741064767",
		"unix milliseconds": "1741064767000",
		"RFC3339":           "2025-03-04T05:06:07Z",
		"RFC3339 nano":      "2025-03-04T05:06:07.000000000Z",
		"SQL":               "2025-03-04 05:06:07",
		"compact":           "20250304050607",
		"time string":       "2025-03-04 05:06:07 +0000 UTC",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ConvertDateStringToTime(input); !got.Equal(want) {
				t.Errorf("ConvertDateStringToTime(%q) = %v, want %v", input, got, want)
			}
		})
	}

	if got := ConvertDateStringToTime("2025-03-04"); !got.Equal(time.Date(2025, 3, 4, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date only = %v", got)
	}
	// Dates that can't be read are now
	for _, input := range []string{"", "0", "undefined", "not a date"} {
		if got := ConvertDateStringToTime(input); time.Since(got) > time.Minute {
			t.Errorf("ConvertDateStringToTime(%q) = %v, want now", input, got)
		}
	}
}

func TestFormatting(t *testing.T) {
	for n, want := range map[int]string{0: "00", 7: "07", 10: "10", 123: "123"} {
		if got := FormatIntAsTwoDigitString(n); got != want {
			t.Errorf("FormatIntAsTwoDigitString(%d) = %q, want %q", n, got, want)
		}
	}
	if ParseFileSize("2048") != 2048 || ParseFileSize("big") != 0 {
		t.Error("ParseFileSize should read a number and return 0 for anything else")
	}
}

func TestGetFileDownloadName(t *testing.T) {
	tests := []struct {
		image models.ImageFile
		want  string
	}{
		{models.ImageFile{Type: "poster"}, "Poster"},
		{models.ImageFile{Type: "backdrop"}, "Backdrop"},
		{models.ImageFile{Type: "season_poster", SeasonNumber: num(1)}, "Season 01 Poster"},
		{models.ImageFile{Type: "season_poster", SeasonNumber: num(0)}, "Special Season Poster"},
		{models.ImageFile{Type: "season_poster"}, "Season Poster"},
		{models.ImageFile{Type: "titlecard", SeasonNumber: num(2), EpisodeNumber: num(3)}, "S02E03 Titlecard"},
		{models.ImageFile{Type: "titlecard", SeasonNumber: num(2)}, "S02 Titlecard"},
		{models.ImageFile{Type: "titlecard", EpisodeNumber: num(3)}, "E03 Titlecard"},
		{models.ImageFile{Type: "titlecard", Title: "Pilot"}, "Pilot Titlecard"},
		{models.ImageFile{Type: "titlecard"}, "Titlecard"},
		{models.ImageFile{Type: "logo"}, "Movie - Image"},
	}
	for _, tt := range tests {
		if got := GetFileDownloadName("Movie", tt.image); got != tt.want {
			t.Errorf("GetFileDownloadName(%+v) = %q, want %q", tt.image, got, tt.want)
		}
	}
}

func TestMediaItemInfo(t *testing.T) {
	got := MediaItemInfo(models.MediaItem{Title: "Movie (2020)", LibraryTitle: "Movies", TMDB_ID: "1", RatingKey: "10"})
	if want := "'Movie' (2020) | Movies [TMDB: 1 | Key: 10]"; got != want {
		t.Errorf("MediaItemInfo = %q, want %q", got, want)
	}
	if got := MediaItemInfo(models.MediaItem{}); got != "'<no title>' (-1) | <no library> [TMDB: <no tmdb> | Key: <no key>]" {
		t.Errorf("MediaItemInfo of an empty item = %q", got)
	}
	if got := CollectionItemInfo(models.CollectionItem{Title: "Saga", LibraryTitle: "Movies", RatingKey: "5"}); got != "'Saga' | Movies [Key: 5]" {
		t.Errorf("CollectionItemInfo = %q", got)
	}
}

func TestRenderTemplate(t *testing.T) {
	vars := MergeTemplateVars(map[string]string{"Title": "Old"}, map[string]string{"Title": "Movie", "Year": "2020"})
	got := RenderTemplate("{{Title}} ({{ Year }}) {{Unknown}}", vars)
	if want := "Movie (2020) {{Unknown}}"; got != want {
		t.Errorf("RenderTemplate = %q, want %q", got, want)
	}
	if RenderTemplate("", vars) != "" {
		t.Error("an empty template should stay empty")
	}
	if _, ok := BaseTemplateVars()["Timestamp"]; !ok {
		t.Error("the base variables should include the timestamp")
	}
}

func TestUpdatePosterSetInDBItem(t *testing.T) {
	sets := []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "1", Title: "Old"}}}}
	found, updated := UpdatePosterSetInDBItem(sets, models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "1", Title: "New"}}})
	if !found || updated[0].Title != "New" {
		t.Errorf("the set should be replaced, got found %v and %+v", found, updated)
	}
	if found, _ := UpdatePosterSetInDBItem(sets, models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "2"}}}); found {
		t.Error("a set that isn't in the list should not be found")
	}
}

func TestFiles(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "a", "b")
	if Err := CreateFolderIfNotExists(testContext(), folder); Err.Message != "" || !fileExists(folder) {
		t.Fatalf("the folder should be created: %s", Err.Message)
	}
	for _, name := range []string{"a/one.jpg", "a/b/two.jpg", "three.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cleared, Err := ClearAllFilesFromFolder(testContext(), dir)
	if Err.Message != "" || cleared != 3 {
		t.Errorf("cleared %d files (%s), want 3", cleared, Err.Message)
	}
	if fileExists(filepath.Join(dir, "three.jpg")) {
		t.Error("the files should be deleted")
	}
	if _, Err := ClearAllFilesFromFolder(testContext(), filepath.Join(dir, "missing")); Err.Message == "" {
		t.Error("clearing a folder that doesn't exist should fail")
	}

	for contentType, want := range map[string]string{"image/png": ".png", "image/webp": ".webp", "image/gif": ".gif", "image/jpeg": ".jpg", "": ".jpg"} {
		if got := GetExtensionFromContentType(contentType); got != want {
			t.Errorf("GetExtensionFromContentType(%q) = %q, want %q", contentType, got, want)
		}
	}
}
