package mediux

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

func TestUserFollowingAndHidingIsKept(t *testing.T) {
	calls := 0
	previousFetch, previousToken := fetchFollowingHide, config.Current.Mediux.ApiToken
	t.Cleanup(func() {
		fetchFollowingHide, config.Current.Mediux.ApiToken = previousFetch, previousToken
		followHideUsers, followHideToken, followHideFetched = nil, "", time.Time{}
	})
	fetchFollowingHide = func(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
		calls++
		return []models.MediuxUserInfo{{Username: "creator", Follow: true}}, logging.LogErrorInfo{}
	}
	config.Current.Mediux.ApiToken = "token"
	ctx := context.Background()

	for range 3 {
		if users, Err := GetUserFollowingAndHiding(ctx); Err.Message != "" || len(users) != 1 {
			t.Fatalf("users = %+v, %s", users, Err.Message)
		}
	}
	if calls != 1 {
		t.Errorf("MediUX was asked %d times, want once", calls)
	}

	// A new token gets the list again
	config.Current.Mediux.ApiToken = "new token"
	_, _ = GetUserFollowingAndHiding(ctx)
	if calls != 2 {
		t.Errorf("after a new token MediUX was asked %d times, want twice", calls)
	}

	// An old list is got again
	followHideFetched = time.Now().Add(-followHideMaxAge - time.Second)
	_, _ = GetUserFollowingAndHiding(ctx)
	if calls != 3 {
		t.Errorf("after the list got old MediUX was asked %d times, want 3", calls)
	}
}
