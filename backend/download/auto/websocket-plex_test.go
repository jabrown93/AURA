package autodownload

import "testing"

// These tests cover the pure Plex timeline-message parsing helpers only. The
// dial/read/reconnect loop in connectAndListenPlexWithStop talks to a real Plex
// server and isn't exercised here — see the migration PR notes for that gap.

func TestIsCompletedMetadataState(t *testing.T) {
	tests := []struct {
		name          string
		metadataState string
		timelineState int
		want          bool
	}{
		{"processed string state", "processed", 0, true},
		{"finished string state", "finished", 0, true},
		{"numeric state 5", "", 5, true},
		{"in-progress numeric state", "", 2, false},
		{"unrecognized string state", "queued", 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCompletedMetadataState(tt.metadataState, tt.timelineState); got != tt.want {
				t.Errorf("isCompletedMetadataState(%q, %d) = %v, want %v", tt.metadataState, tt.timelineState, got, tt.want)
			}
		})
	}
}

func TestBuildPlexRefreshMessage(t *testing.T) {
	entry := map[string]any{
		"metadataState": "processed",
		"title":         "Some Show S01 E02",
		"sectionID":     float64(7),
		"type":          float64(4),
		"itemID":        "12345",
	}

	msg, ok := buildPlexRefreshMessage(entry)
	if !ok {
		t.Fatalf("buildPlexRefreshMessage() returned ok=false, want true")
	}
	if msg.SectionID != 7 {
		t.Errorf("SectionID = %d, want 7", msg.SectionID)
	}
	if msg.ItemType != "episode" {
		t.Errorf("ItemType = %q, want %q", msg.ItemType, "episode")
	}
	if msg.ItemRatingKey != "12345" {
		t.Errorf("ItemRatingKey = %q, want %q", msg.ItemRatingKey, "12345")
	}

	// A non-completed state should be filtered out.
	entry["metadataState"] = "queued"
	if _, ok := buildPlexRefreshMessage(entry); ok {
		t.Errorf("buildPlexRefreshMessage() with queued state: ok = true, want false")
	}

	// Missing sectionID should be filtered out.
	entry["metadataState"] = "processed"
	delete(entry, "sectionID")
	if _, ok := buildPlexRefreshMessage(entry); ok {
		t.Errorf("buildPlexRefreshMessage() with no sectionID: ok = true, want false")
	}
}

func TestBuildPlexRefreshMessageFromTyped(t *testing.T) {
	entry := PlexTimelineEntry{
		SectionID:     "3",
		ItemID:        "999",
		Type:          1,
		Title:         "Some Movie",
		State:         5,
		MetadataState: "",
	}

	msg, ok := buildPlexRefreshMessageFromTyped(entry)
	if !ok {
		t.Fatalf("buildPlexRefreshMessageFromTyped() returned ok=false, want true")
	}
	if msg.ItemType != "movie" {
		t.Errorf("ItemType = %q, want %q", msg.ItemType, "movie")
	}
	if msg.SectionID != 3 {
		t.Errorf("SectionID = %d, want 3", msg.SectionID)
	}
}

func TestExtractCompletedPlexRefreshes(t *testing.T) {
	payload := []byte(`{
		"NotificationContainer": {
			"type": "timeline",
			"TimelineEntry": [
				{
					"metadataState": "processed",
					"sectionID": 7,
					"ratingKey": "111",
					"grandparentRatingKey": "222",
					"grandparentTitle": "Some Show",
					"parentIndex": 1,
					"index": 2
				},
				{
					"metadataState": "queued",
					"sectionID": 7,
					"ratingKey": "333"
				}
			]
		}
	}`)

	refreshes, err := extractCompletedPlexRefreshes(payload)
	if err != nil {
		t.Fatalf("extractCompletedPlexRefreshes() error = %v", err)
	}
	if len(refreshes) != 1 {
		t.Fatalf("len(refreshes) = %d, want 1", len(refreshes))
	}

	got := refreshes[0]
	if got.EpisodeRatingKey != "111" {
		t.Errorf("EpisodeRatingKey = %q, want %q", got.EpisodeRatingKey, "111")
	}
	if got.SeriesRatingKey != "222" {
		t.Errorf("SeriesRatingKey = %q, want %q", got.SeriesRatingKey, "222")
	}
	if got.Subtitle != "Some Show S01 E02" {
		t.Errorf("Subtitle = %q, want %q", got.Subtitle, "Some Show S01 E02")
	}
}

func TestExtractShowTitleAndNormalizeTitle(t *testing.T) {
	tests := []struct {
		subtitle string
		want     string
	}{
		{"The Show S01 E02", "The Show"},
		{"The Show S01", "The Show"},
		{"The Movie (2024)", "The Movie"},
		{"The Movie 2024", "The Movie"},
		{"Plain Title", "Plain Title"},
	}

	for _, tt := range tests {
		if got := extractShowTitle(tt.subtitle); got != tt.want {
			t.Errorf("extractShowTitle(%q) = %q, want %q", tt.subtitle, got, tt.want)
		}
	}

	if got := normalizeTitle("The, Show!! 123"); got != "the show 123" {
		t.Errorf("normalizeTitle() = %q, want %q", got, "the show 123")
	}
}

func TestParseRatingKeyFromPath(t *testing.T) {
	if got := parseRatingKeyFromPath("/library/metadata/456"); got != "456" {
		t.Errorf("parseRatingKeyFromPath() = %q, want %q", got, "456")
	}
	if got := parseRatingKeyFromPath(""); got != "" {
		t.Errorf("parseRatingKeyFromPath(\"\") = %q, want empty", got)
	}
}
