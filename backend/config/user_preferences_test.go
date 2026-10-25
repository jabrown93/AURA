package config

import (
	"aura/logging"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestValidateUserPreferencesFavoriteCreators(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	prefs := Config_UserPreferences{FavoriteCreators: []string{" willtong93 ", "", "benno", "WillTong93", "Pejamas"}}
	if !ValidateUserPreferences(ctx, &prefs) {
		t.Fatal("the preferences should be valid")
	}
	// Blank and repeated names go, each keeps its best rank
	if want := []string{"willtong93", "benno", "Pejamas"}; !slices.Equal(prefs.FavoriteCreators, want) {
		t.Errorf("favorite creators = %q, want %q", prefs.FavoriteCreators, want)
	}
}

func TestValidateUserPreferencesPageViews(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	home := map[string]any{"sortOption": "title", "sortOrder": "asc", "filteredLibraries": []any{"Movies"}}
	prefs := Config_UserPreferences{PageViews: map[string]map[string]any{
		"home":     home,
		"unknown":  {"sortOption": "title"},
		"logs":     {"levelsFilter": strings.Repeat("x", maxPageViewSize)},
		"user_set": {"sortOption": "title"},
	}}
	if !ValidateUserPreferences(ctx, &prefs) {
		t.Fatal("the preferences should be valid")
	}
	// Known pages are kept, unknown pages and views that are too large are removed
	if len(prefs.PageViews) != 1 || prefs.PageViews["home"]["sortOption"] != "title" {
		t.Errorf("page views = %v", prefs.PageViews)
	}
}
