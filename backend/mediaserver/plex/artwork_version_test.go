package plex

import (
	"testing"
	"time"
)

// Guards the unit contract: Plex reports updatedAt in Unix seconds, but artwork
// versions are compared against a microsecond generation floor. Returning the raw
// seconds value puts it ~6 orders of magnitude below the floor, where it clamps and
// a poster changed directly in Plex never busts the browser cache.
func TestArtworkVersionConvertsPlexSecondsToMicroseconds(t *testing.T) {
	changed := time.Date(2026, 8, 31, 13, 0, 0, 0, time.UTC)

	if got, want := artworkVersion(changed.Unix()), changed.UnixMicro(); got != want {
		t.Fatalf("artworkVersion(%d) = %d, want %d", changed.Unix(), got, want)
	}
	if got := artworkVersion(0); got != 0 {
		t.Fatalf("artworkVersion(0) = %d, want 0 so the cache applies its floor", got)
	}

	processStart := changed.Add(-time.Hour).UnixMicro()
	if artworkVersion(changed.Unix()) <= processStart {
		t.Fatalf("converted version %d must exceed a floor of %d to bust caches",
			artworkVersion(changed.Unix()), processStart)
	}
}
