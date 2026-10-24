package autodownload

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"testing"
	"time"
)

func TestRankSets(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	previousFollow, previousPrefs := getFollowing, config.Current.UserPreferences
	t.Cleanup(func() {
		getFollowing = previousFollow
		config.Current.UserPreferences = previousPrefs
	})
	getFollowing = func(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
		return []models.MediuxUserInfo{{Username: "followed", Follow: true}}, logging.LogErrorInfo{}
	}
	config.Current.UserPreferences = config.Config_UserPreferences{FavoriteCreators: []string{"first", "Second"}}

	sets := []models.SetRef{
		testSet("other-unranked-new", "other", "movie", 0, day(9)),
		testSet("other-rank-5", "other", "movie", 5, day(1)),
		testSet("other-unranked-old", "other", "movie", 0, day(1)),
		testSet("followed", "followed", "movie", 1, day(1)),
		testSet("second", "second", "movie", 1, day(1)),
		testSet("first-rank-20", "first", "movie", 20, day(1)),
		testSet("first-rank-3", "first", "movie", 3, day(1)),
		testSet("saved", "other", "movie", 0, day(1)),
	}
	item := models.MediaItem{TMDB_ID: "1", Type: "movie", DBSavedSets: []models.DBSavedSet{{ID: "saved"}}}

	ranked := RankSets(testContext(), item, sets)
	got := []string{}
	for _, set := range ranked {
		got = append(got, set.ID)
	}
	want := []string{"saved", "first-rank-3", "first-rank-20", "second", "followed", "other-rank-5", "other-unranked-new", "other-unranked-old"}
	if !slices.Equal(got, want) {
		t.Errorf("RankSets = %v, want %v", got, want)
	}
	if sets[0].ID != "other-unranked-new" {
		t.Error("RankSets should not reorder the sets it was given")
	}
}
