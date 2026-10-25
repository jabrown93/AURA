package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"strings"
)

// Set in tests
var getSavedSets = database.GetAllSavedSets

// UpgradeSavedSetsResult is what a run did
type UpgradeSavedSetsResult struct {
	Checked  int `json:"checked"`  // Saved items that can be upgraded
	Upgraded int `json:"upgraded"` // Items a better set was added to the Download Queue for
}

// UpgradeItem is a saved item to upgrade
type UpgradeItem struct {
	TMDB_ID      string `json:"tmdb_id"`
	LibraryTitle string `json:"library_title"`
	Edition      string `json:"edition"`
}

// UpgradeSavedSets upgrades every saved set, while Automatically Upgrade Sets is on
func UpgradeSavedSets(ctx context.Context) (result UpgradeSavedSetsResult, Err logging.LogErrorInfo) {
	if !config.Current.UserPreferences.AutoUpgradeSets {
		_, logAction := logging.AddSubActionToContext(ctx, "Upgrading Saved Sets", logging.LevelInfo)
		logAction.AppendResult("skipped", "Automatically Upgrade Sets is off")
		logAction.Complete()
		return result, logging.LogErrorInfo{}
	}
	return UpgradeSets(ctx, nil)
}

// UpgradeSets replaces saved sets with a set from a higher ranked favorite creator, for the items given or every saved item.
// Any favorite creator ranks above a creator who isn't one. An item with a set that has Auto Upgrade off is kept.
// The new set uses the Download Defaults with Auto Download on, and replaces the item's sets once it's saved.
func UpgradeSets(ctx context.Context, only []UpgradeItem) (result UpgradeSavedSetsResult, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Upgrading Saved Sets", logging.LevelInfo)
	defer logAction.Complete()

	favoriteCreatorsMu.Lock()
	defer favoriteCreatorsMu.Unlock()

	prefs := config.Current.UserPreferences
	favorites := favoriteCreators(ctx, prefs.FavoriteCreators)
	if len(favorites) == 0 {
		logAction.AppendResult("skipped", "there are no favorite creators")
		return result, logging.LogErrorInfo{}
	}

	saved, Err := getSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" {
		return result, Err
	}
	setsByRank := favoriteSetsByRank(ctx, logAction, favorites)
	queued := queuedItems(ctx)

	// rankOf is the creator's rank from 0, a creator who isn't a favorite ranks last
	rankOf := func(username string) int {
		for rank, favorite := range favorites {
			if strings.EqualFold(favorite, username) {
				return rank
			}
		}
		return len(favorites)
	}

	wanted := map[string]bool{}
	for _, item := range only {
		wanted[item.TMDB_ID+"|"+item.LibraryTitle+"|"+item.Edition] = true
	}

	selected := DownloadDefaultTypes()
	for _, item := range saved.Items {
		if len(only) > 0 && !wanted[item.MediaItem.TMDB_ID+"|"+item.MediaItem.LibraryTitle+"|"+item.MediaItem.Edition] {
			continue
		}
		if len(item.PosterSets) == 0 || queued[item.MediaItem.TMDB_ID+"|"+item.MediaItem.LibraryTitle+"|"+item.MediaItem.Edition] {
			continue
		}
		// The best ranked creator of the saved sets, an item with a set that's kept isn't upgraded
		currentRank := len(favorites)
		keep := false
		for _, set := range item.PosterSets {
			keep = keep || set.NoAutoUpgrade
			currentRank = min(currentRank, rankOf(set.UserCreated))
		}
		if keep {
			continue
		}
		result.Checked++

		best, bestRank, found := bestSet(item.MediaItem, setsByRank)
		if !found || bestRank >= currentRank {
			continue
		}

		replaced := []string{}
		for _, set := range item.PosterSets {
			replaced = append(replaced, set.ID)
		}
		Err := addToQueue(ctx, models.DBSavedItem{
			MediaItem: item.MediaItem,
			PosterSets: []models.DBPosterSetDetail{{
				PosterSet:     best.PosterSet,
				SelectedTypes: selected,
				AutoDownload:  true,
			}},
			Source:        models.SourceAutoUpgrade,
			ReplaceSetIDs: replaced,
		})
		if Err.Message != "" {
			logAction.AppendWarning(fmt.Sprintf("%s_queue", item.MediaItem.Title), Err.Message)
			continue
		}
		result.Upgraded++
		logAction.AppendResult(item.MediaItem.Title, fmt.Sprintf("%s by %s replaces %s", best.Title, best.UserCreated, strings.Join(replaced, ", ")))
	}

	logAction.AppendResult("checked", result.Checked)
	logAction.AppendResult("upgraded", result.Upgraded)
	return result, logging.LogErrorInfo{}
}
