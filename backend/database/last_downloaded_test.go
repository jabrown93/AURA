package database

import (
	"testing"
	"time"
)

func TestLastDownloaded(t *testing.T) {
	if got := lastDownloaded(time.Time{}); time.Since(got) > time.Minute {
		t.Errorf("a set saved without a date should be saved now, got %v", got)
	}
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := lastDownloaded(when); !got.Equal(when) {
		t.Errorf("a set's date should be kept, got %v", got)
	}
}
