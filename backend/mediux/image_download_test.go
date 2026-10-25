package mediux

import (
	"bytes"
	"os"
	"testing"
)

func TestImageCache(t *testing.T) {
	folder := t.TempDir()

	if _, _, found := readCachedImage(folder, "asset_20260101000000"); found {
		t.Fatal("an image that was never cached should not be found")
	}

	cacheImage(folder, "asset_20260101000000.png", []byte("png data"))
	data, imageType, found := readCachedImage(folder, "asset_20260101000000")
	if !found || imageType != "image/png" || !bytes.Equal(data, []byte("png data")) {
		t.Errorf("cached image = %q, %q, found %v", data, imageType, found)
	}

	// Only the image is left in the folder
	if entries, _ := os.ReadDir(folder); len(entries) != 1 {
		t.Errorf("files in the cache folder = %d, want 1", len(entries))
	}
}
