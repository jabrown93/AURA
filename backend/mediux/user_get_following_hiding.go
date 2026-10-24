package mediux

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
	"slices"
	"sync"
	"time"
)

//go:embed gen_user_follow_hide.graphql
var queryUserFollowHideQuery string

// The followed and hidden creators are kept for a few minutes
const followHideMaxAge = 5 * time.Minute

var (
	followHideMu       sync.Mutex
	followHideUsers    []models.MediuxUserInfo
	followHideToken    string // The MediUX token the list was got with
	followHideFetched  time.Time
	fetchFollowingHide = fetchUserFollowingAndHiding
)

// GetUserFollowingAndHiding returns the creators the user follows and hides on MediUX
func GetUserFollowingAndHiding(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
	followHideMu.Lock()
	defer followHideMu.Unlock()

	token := config.Current.Mediux.ApiToken
	if followHideToken == token && !followHideFetched.IsZero() && time.Since(followHideFetched) < followHideMaxAge {
		return slices.Clone(followHideUsers), logging.LogErrorInfo{}
	}

	users, Err := fetchFollowingHide(ctx)
	if Err.Message != "" {
		return users, Err
	}
	followHideUsers, followHideToken, followHideFetched = users, token, time.Now()
	return slices.Clone(users), logging.LogErrorInfo{}
}

func fetchUserFollowingAndHiding(ctx context.Context) (userFollowHide []models.MediuxUserInfo, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Fetching User Following and Hiding Information", logging.LevelDebug)
	defer logAction.Complete()

	userFollowHide = []models.MediuxUserInfo{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryUserFollowHideQuery,
		Variables: map[string]any{},
		QueryName: "getUserFollowHide",
	})
	if Err.Message != "" {
		return userFollowHide, Err
	}

	type MediuxUserAvatar struct {
		ID string `json:"id"`
	}

	type MediuxFollowHideUserInfo struct {
		ID       string           `json:"id"`
		Username string           `json:"username"`
		Avatar   MediuxUserAvatar `json:"avatar"`
	}

	type MediuxUserFollow struct {
		FolloweeID MediuxFollowHideUserInfo `json:"followee_id"`
	}

	type MediuxUserHide struct {
		HidingID MediuxFollowHideUserInfo `json:"hiding_id"`
	}

	type MediuxUserFollowHideResponse struct {
		Data struct {
			Follows []MediuxUserFollow `json:"user_follows,omitempty"`
			Hides   []MediuxUserHide   `json:"user_hides,omitempty"`
		} `json:"data"`
	}

	// Decode the response
	var gqlResponse MediuxUserFollowHideResponse
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &gqlResponse, "MediUX User Follow/Hide Response Decoding")
	if Err.Message != "" {
		return userFollowHide, Err
	}

	data := gqlResponse.Data
	if data.Follows == nil && data.Hides == nil {
		return nil, Err
	}

	// Populate the UserFollowHide object with the response data
	if data.Follows != nil {
		for _, follow := range data.Follows {
			userFollowHide = append(userFollowHide, models.MediuxUserInfo{
				ID:       follow.FolloweeID.ID,
				Username: follow.FolloweeID.Username,
				Avatar:   follow.FolloweeID.Avatar.ID,
				Follow:   true,
			})
		}
	}
	if data.Hides != nil {
		for _, hide := range data.Hides {
			userFollowHide = append(userFollowHide, models.MediuxUserInfo{
				ID:       hide.HidingID.ID,
				Username: hide.HidingID.Username,
				Avatar:   hide.HidingID.Avatar.ID,
				Hide:     true,
			})
		}
	}

	// Remove duplicates and if both follow and hide exist, prioritize follow
	uniqueUsers := make(map[string]models.MediuxUserInfo)
	for _, user := range userFollowHide {
		existingUser, exists := uniqueUsers[user.ID]
		if exists {
			// If the user already exists, update the follow/hide status accordingly
			existingUser.Follow = existingUser.Follow || user.Follow
			existingUser.Hide = existingUser.Hide || user.Hide
			uniqueUsers[user.ID] = existingUser
		} else {
			uniqueUsers[user.ID] = user
		}
	}

	// Convert the map back to a slice
	userFollowHide = make([]models.MediuxUserInfo, 0, len(uniqueUsers))
	for _, user := range uniqueUsers {
		userFollowHide = append(userFollowHide, user)
	}

	return userFollowHide, logging.LogErrorInfo{}
}
