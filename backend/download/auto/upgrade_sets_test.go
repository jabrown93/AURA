package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"testing"
	"time"
)

func TestUpgradeSavedSets(t *testing.T) {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	creatorSets := map[string]models.CreatorSetsResponse{
		"first":  {MovieSets: []models.SetRef{testSet("first-1", "first", "movie", 0, day, "1"), testSet("first-3", "first", "movie", 0, day, "3")}},
		"second": {MovieSets: []models.SetRef{testSet("second-2", "second", "movie", 0, day, "2", "4", "5", "6")}},
	}
	savedSet := func(id, user string, keep bool) models.DBPosterSetDetail {
		return models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, UserCreated: user, Type: "movie"}}, NoAutoUpgrade: keep}
	}
	savedItem := func(tmdbID string, sets ...models.DBPosterSetDetail) models.DBSavedItem {
		return models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: tmdbID, Type: "movie", Title: "Item " + tmdbID, LibraryTitle: "Movies"}, PosterSets: sets}
	}
	saved := []models.DBSavedItem{
		savedItem("1", savedSet("old-1", "second", false)),                                     // A higher ranked favorite has a set
		savedItem("2", savedSet("old-2", "someone", false), savedSet("old-2b", "else", false)), // A favorite beats creators who aren't favorites, both sets are replaced
		savedItem("3", savedSet("first-3", "first", false)),                                    // Already the best ranked creator
		savedItem("4", savedSet("old-4", "someone", true)),                                     // Auto Upgrade off
		savedItem("5", savedSet("old-5", "someone", false)),                                    // Already in the Download Queue
		savedItem("6", savedSet("old-6", "first", false)),                                      // Saved from a higher ranked creator than the best available
	}

	queued := []models.DBSavedItem{}
	previous := struct {
		sets   func(context.Context, string) (models.CreatorSetsResponse, logging.LogErrorInfo)
		follow func(context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo)
		jobs   func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo)
		add    func(context.Context, models.DBSavedItem) logging.LogErrorInfo
		saved  func(context.Context, models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo)
		prefs  config.Config_UserPreferences
	}{getCreatorSets, getFollowing, getQueueJobs, addToQueue, getSavedSets, config.Current.UserPreferences}
	t.Cleanup(func() {
		getCreatorSets, getFollowing, getQueueJobs, addToQueue, getSavedSets = previous.sets, previous.follow, previous.jobs, previous.add, previous.saved
		config.Current.UserPreferences = previous.prefs
	})
	getCreatorSets = func(ctx context.Context, username string) (models.CreatorSetsResponse, logging.LogErrorInfo) {
		return creatorSets[username], logging.LogErrorInfo{}
	}
	getFollowing = func(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
		return nil, logging.LogErrorInfo{}
	}
	getQueueJobs = func(ctx context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return []database.DownloadQueueJob{{TMDB_ID: "5", LibraryTitle: "Movies", Status: "pending"}}, logging.LogErrorInfo{}
	}
	addToQueue = func(ctx context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		queued = append(queued, item)
		return logging.LogErrorInfo{}
	}
	getSavedSets = func(ctx context.Context, _ models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo) {
		return database.PagedSavedItems{Items: saved}, logging.LogErrorInfo{}
	}
	config.Current.UserPreferences = config.Config_UserPreferences{
		AutoUpgradeSets:  true,
		FavoriteCreators: []string{"first", "second"},
		DownloadDefaults: []string{"poster", "backdrop", "titlecard"},
	}

	result, Err := UpgradeSavedSets(testContext())
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if result != (UpgradeSavedSetsResult{Checked: 4, Upgraded: 2}) {
		t.Errorf("result = %+v", result)
	}
	got := map[string]models.DBSavedItem{}
	for _, item := range queued {
		got[item.MediaItem.TMDB_ID] = item
	}
	if len(got) != 2 || got["1"].PosterSets[0].ID != "first-1" || got["2"].PosterSets[0].ID != "second-2" {
		t.Fatalf("queued = %+v", queued)
	}
	if !slices.Equal(got["1"].ReplaceSetIDs, []string{"old-1"}) || !slices.Equal(got["2"].ReplaceSetIDs, []string{"old-2", "old-2b"}) {
		t.Errorf("replaced = %v and %v", got["1"].ReplaceSetIDs, got["2"].ReplaceSetIDs)
	}
	// The Download Defaults with Auto Download on
	if set := got["1"].PosterSets[0]; !set.AutoDownload || !set.SelectedTypes.Poster || !set.SelectedTypes.Titlecard || got["1"].Source != models.SourceAutoUpgrade {
		t.Errorf("upgraded set = %+v, source %q", set, got["1"].Source)
	}

	// Nothing happens while Automatically Upgrade Sets is off
	config.Current.UserPreferences.AutoUpgradeSets = false
	queued = nil
	if result, _ := UpgradeSavedSets(testContext()); result.Checked != 0 || len(queued) != 0 {
		t.Errorf("with upgrades off: %+v, queued %d", result, len(queued))
	}

	// Chosen items are upgraded even with the switch off, and only those
	result, Err = UpgradeSets(testContext(), []UpgradeItem{{TMDB_ID: "2", LibraryTitle: "Movies"}, {TMDB_ID: "4", LibraryTitle: "Movies"}})
	if Err.Message != "" || result != (UpgradeSavedSetsResult{Checked: 1, Upgraded: 1}) || len(queued) != 1 || queued[0].MediaItem.TMDB_ID != "2" {
		t.Errorf("chosen items: %+v %s, queued %+v", result, Err.Message, queued)
	}
}
