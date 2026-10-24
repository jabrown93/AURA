package autodownload

import (
	"aura/config"
	"aura/models"
	"context"
	"slices"
	"strings"
)

// RankSets orders an item's sets: saved sets first, then favorite creator rank, then MediUX popularity, then newest
func RankSets(ctx context.Context, item models.MediaItem, sets []models.SetRef) []models.SetRef {
	saved := map[string]bool{}
	for _, set := range item.DBSavedSets {
		saved[set.ID] = true
	}

	// A favorite's rank is its place in the list, creators that aren't favorites come after them
	favorites := favoriteCreators(ctx, config.Current.UserPreferences.FavoriteCreators)
	creatorRank := func(username string) int {
		if i := slices.IndexFunc(favorites, func(name string) bool { return strings.EqualFold(name, username) }); i >= 0 {
			return i
		}
		return len(favorites)
	}

	ranked := slices.Clone(sets)
	slices.SortStableFunc(ranked, func(a, b models.SetRef) int {
		if saved[a.ID] != saved[b.ID] {
			if saved[a.ID] {
				return -1
			}
			return 1
		}
		if byCreator := creatorRank(a.UserCreated) - creatorRank(b.UserCreated); byCreator != 0 {
			return byCreator
		}
		if byRank := comparePopularity(a.PopularityGlobal, b.PopularityGlobal); byRank != 0 {
			return byRank
		}
		return b.DateUpdated.Compare(a.DateUpdated)
	})
	return ranked
}
