package siloplugin

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func number(n int) *int { return &n }

func testSet(id, user string, images ...models.ImageFile) models.SetRef {
	return models.SetRef{PosterSet: models.PosterSet{
		BaseSetInfo: models.BaseSetInfo{ID: id, Title: id, Type: "movie", UserCreated: user},
		Images:      images,
	}}
}

// useTestSecret signs tokens with a fixed secret
func useTestSecret(t *testing.T, secret string) {
	previous := getAuthSecret
	t.Cleanup(func() { getAuthSecret = previous })
	getAuthSecret = func(ctx context.Context) (string, error) { return secret, nil }
}

// useFavorites sets the favorite creators
func useFavorites(t *testing.T, usernames ...string) {
	previous := getFavorites
	t.Cleanup(func() { getFavorites = previous })
	getFavorites = func(context.Context) []string { return usernames }
}

// useMovieSets serves the sets for every movie, in the order given
func useMovieSets(t *testing.T, movieSets, collectionSets []models.SetRef) {
	previous := struct {
		movie, collection func(context.Context, string, string, string, *map[string]models.IncludedItem) ([]models.SetRef, logging.LogErrorInfo)
		rank              func(context.Context, models.MediaItem, []models.SetRef) []models.SetRef
		preview, original func(context.Context, models.ImageFile) (string, logging.LogErrorInfo)
		check             func(context.Context) (int, logging.LogErrorInfo)
	}{getMovieSets, getCollectionSets, rankSets, previewURL, originalURL, checkNewItems}
	t.Cleanup(func() {
		getMovieSets, getCollectionSets, rankSets, previewURL, originalURL, checkNewItems = previous.movie, previous.collection, previous.rank, previous.preview, previous.original, previous.check
	})
	originalURL = func(_ context.Context, image models.ImageFile) (string, logging.LogErrorInfo) {
		return "https://original/" + image.ID, logging.LogErrorInfo{}
	}
	checkNewItems = func(context.Context) (int, logging.LogErrorInfo) { return 0, logging.LogErrorInfo{} }
	clone := func(sets []models.SetRef) []models.SetRef {
		out := slices.Clone(sets)
		for i := range out {
			out[i].Images = slices.Clone(out[i].Images)
		}
		return out
	}
	getMovieSets = func(context.Context, string, string, string, *map[string]models.IncludedItem) ([]models.SetRef, logging.LogErrorInfo) {
		return clone(movieSets), logging.LogErrorInfo{}
	}
	getCollectionSets = func(context.Context, string, string, string, *map[string]models.IncludedItem) ([]models.SetRef, logging.LogErrorInfo) {
		return clone(collectionSets), logging.LogErrorInfo{}
	}
	rankSets = func(_ context.Context, _ models.MediaItem, sets []models.SetRef) []models.SetRef { return sets }
	previewURL = func(_ context.Context, image models.ImageFile) (string, logging.LogErrorInfo) {
		return "https://preview/" + image.ID, logging.LogErrorInfo{}
	}
}

func useLibrary(t *testing.T, sections ...models.LibrarySection) {
	cache.LibraryStore.ClearAllSections()
	t.Cleanup(cache.LibraryStore.ClearAllSections)
	for i := range sections {
		cache.LibraryStore.UpdateSection(&sections[i])
	}
}

func TestDownloadToken(t *testing.T) {
	useTestSecret(t, "secret")
	ctx := testContext()
	token := DownloadToken{TMDB_ID: "1", ItemType: "show", SetID: "set", ImageID: "image", ImageType: "titlecard",
		Modified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), SeasonNumber: number(1), EpisodeNumber: number(2)}

	signed, err := SignDownloadToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyDownloadToken(ctx, signed)
	if err != nil {
		t.Fatal(err)
	}
	if got.SetID != "set" || got.ImageType != "titlecard" || *got.EpisodeNumber != 2 || !got.Modified.Equal(token.Modified) {
		t.Errorf("VerifyDownloadToken = %+v, want %+v", got, token)
	}

	// A changed payload or signature is refused
	payload, signature, _ := strings.Cut(signed, ".")
	other, _ := SignDownloadToken(ctx, DownloadToken{TMDB_ID: "2"})
	otherPayload, _, _ := strings.Cut(other, ".")
	for name, bad := range map[string]string{
		"other payload":   otherPayload + "." + signature,
		"other signature": payload + "." + signature[:len(signature)-2] + "AA",
		"no signature":    payload,
	} {
		if _, err := VerifyDownloadToken(ctx, bad); err == nil {
			t.Errorf("%s: VerifyDownloadToken should fail", name)
		}
	}

	// A token from another aura is refused
	useTestSecret(t, "another secret")
	if _, err := VerifyDownloadToken(ctx, signed); err == nil {
		t.Error("a token signed with another secret should fail")
	}
}

func TestItemImages(t *testing.T) {
	useTestSecret(t, "secret")
	useLibrary(t,
		models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
			MediaItems: []models.MediaItem{{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies",
				DBSavedSets: []models.DBSavedSet{{ID: "saved", SelectedTypes: models.SelectedTypes{Poster: true}}}}}},
		models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "4K Movies", Type: "movie"},
			MediaItems: []models.MediaItem{{TMDB_ID: "1", Type: "movie", LibraryTitle: "4K Movies",
				DBSavedSets: []models.DBSavedSet{{ID: "saved", SelectedTypes: models.SelectedTypes{Backdrop: true}}}}}},
	)
	useMovieSets(t,
		[]models.SetRef{
			testSet("saved", "first",
				models.ImageFile{ID: "saved-poster", Type: "poster", ItemTMDB_ID: "1"},
				models.ImageFile{ID: "saved-backdrop", Type: "backdrop", ItemTMDB_ID: "1", Language: "English"}),
			testSet("other", "second", models.ImageFile{ID: "other-poster", Type: "poster", ItemTMDB_ID: "1"}),
		},
		[]models.SetRef{testSet("collection", "third",
			models.ImageFile{ID: "collection-poster", Type: "poster", ItemTMDB_ID: "1"},
			models.ImageFile{ID: "other-movie-poster", Type: "poster", ItemTMDB_ID: "2"},
			models.ImageFile{ID: "collection-only", Type: "collection_poster", ItemTMDB_ID: "1"})},
	)

	images, Err := ItemImages(testContext(), "1", "movie", PurposeGallery)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	got := []string{}
	for _, image := range images {
		got = append(got, image.ID)
	}
	want := []string{"saved-poster", "saved-backdrop", "other-poster", "collection-poster"}
	if !slices.Equal(got, want) {
		t.Fatalf("ItemImages = %v, want %v (only this movie's images of known types)", got, want)
	}

	// Saved combines the selected types of every library
	if !images[0].Saved || !images[1].Saved || images[2].Saved {
		t.Errorf("saved = %v %v %v, want true true false", images[0].Saved, images[1].Saved, images[2].Saved)
	}
	if images[0].SetRank != 1 || images[2].SetRank != 2 || images[3].SetRank != 3 {
		t.Errorf("set ranks = %d %d %d, want 1 2 3", images[0].SetRank, images[2].SetRank, images[3].SetRank)
	}
	if images[1].Language != "English" || images[0].PreviewURL != "https://preview/saved-poster" || images[0].OriginalURL != "https://original/saved-poster" {
		t.Errorf("image fields = %+v", images[1])
	}

	// The download path is a token for the image
	token, err := VerifyDownloadToken(testContext(), strings.TrimPrefix(images[2].DownloadPath, "/api/plugin/silo/download/"))
	if err != nil {
		t.Fatal(err)
	}
	if token.ImageID != "other-poster" || token.SetID != "other" || token.TMDB_ID != "1" || token.ItemType != "movie" || token.Creator != "second" || token.Refresh {
		t.Errorf("token = %+v", token)
	}

	// An item aura doesn't have has no images
	if images, Err := ItemImages(testContext(), "9", "movie", PurposeGallery); Err.Message != "" || len(images) != 0 {
		t.Errorf("unknown item = %v %s, want none", images, Err.Message)
	}
}

func TestItemImagesForRefresh(t *testing.T) {
	useTestSecret(t, "secret")
	useFavorites(t, "Favorite")
	useLibrary(t, models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
		MediaItems: []models.MediaItem{{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies",
			DBSavedSets: []models.DBSavedSet{{ID: "saved", SelectedTypes: models.SelectedTypes{Poster: true}}}}}})
	useMovieSets(t, []models.SetRef{
		testSet("saved", "someone",
			models.ImageFile{ID: "saved-poster", Type: "poster"},
			models.ImageFile{ID: "saved-backdrop", Type: "backdrop"}),
		testSet("favorite", "favorite",
			models.ImageFile{ID: "favorite-poster", Type: "poster"},
			models.ImageFile{ID: "favorite-backdrop", Type: "backdrop"}),
		testSet("other", "other",
			models.ImageFile{ID: "other-backdrop", Type: "backdrop"}),
	}, nil)

	images, Err := ItemImages(testContext(), "1", "movie", PurposeRefresh)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	got := []string{}
	for _, image := range images {
		got = append(got, image.ID)
	}
	// The saved poster, and for backdrops, which the saved set doesn't cover, only the favorite's
	if want := []string{"saved-poster", "favorite-backdrop"}; !slices.Equal(got, want) {
		t.Fatalf("refresh images = %v, want %v", got, want)
	}
	token, err := VerifyDownloadToken(testContext(), strings.TrimPrefix(images[1].DownloadPath, "/api/plugin/silo/download/"))
	if err != nil || !token.Refresh || token.Creator != "favorite" {
		t.Errorf("refresh token = %+v %v, want a refresh token for favorite", token, err)
	}

	// Without a favorite creator's set, a refresh is offered nothing
	useFavorites(t)
	cache.LibraryStore.ClearAllSections()
	useLibrary(t, models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
		MediaItems: []models.MediaItem{{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies"}}})
	if images, _ := ItemImages(testContext(), "1", "movie", PurposeRefresh); len(images) != 0 {
		t.Errorf("refresh without favorites = %d images, want none", len(images))
	}
}

func TestQueueSetForDownload(t *testing.T) {
	useLibrary(t, models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Series", Type: "show"},
		MediaItems: []models.MediaItem{
			{TMDB_ID: "1", Type: "show", LibraryTitle: "Series"},
			{TMDB_ID: "2", Type: "show", LibraryTitle: "Series", DBSavedSets: []models.DBSavedSet{{ID: "older"}}},
			{TMDB_ID: "3", Type: "show", LibraryTitle: "Series", DBSavedSets: []models.DBSavedSet{{ID: "set"}}},
			{TMDB_ID: "4", Type: "show", LibraryTitle: "Series"},
			{TMDB_ID: "5", Type: "show", LibraryTitle: "Series", IgnoredInDB: true},
		}})
	showSet := models.SetRef{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set", Type: "show", UserCreated: "creator"},
		Images: []models.ImageFile{{ID: "card", Type: "titlecard", SeasonNumber: number(1), EpisodeNumber: number(1)}}}}

	queued := []models.DBSavedItem{}
	previous := struct {
		show  func(context.Context, string, string, string) ([]models.SetRef, map[string]models.IncludedItem, logging.LogErrorInfo)
		jobs  func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo)
		add   func(context.Context, models.DBSavedItem) logging.LogErrorInfo
		prefs config.Config_UserPreferences
	}{getShowSets, getQueueJobs, addToQueue, config.Current.UserPreferences}
	t.Cleanup(func() {
		getShowSets, getQueueJobs, addToQueue = previous.show, previous.jobs, previous.add
		config.Current.UserPreferences = previous.prefs
	})
	getShowSets = func(context.Context, string, string, string) ([]models.SetRef, map[string]models.IncludedItem, logging.LogErrorInfo) {
		return []models.SetRef{showSet}, nil, logging.LogErrorInfo{}
	}
	getQueueJobs = func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return []database.DownloadQueueJob{{TMDB_ID: "4", LibraryTitle: "Series", Status: "processing"}}, logging.LogErrorInfo{}
	}
	addToQueue = func(_ context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		queued = append(queued, item)
		return logging.LogErrorInfo{}
	}
	config.Current.UserPreferences = config.Config_UserPreferences{DownloadDefaults: []string{"poster", "season_poster", "titlecard"}}

	token := func(tmdbID string) DownloadToken {
		return DownloadToken{TMDB_ID: tmdbID, ItemType: "series", SetID: "set", ImageType: "titlecard", SeasonNumber: number(1), EpisodeNumber: number(1), Creator: "creator", Refresh: true}
	}
	useFavorites(t, "Creator")
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		QueueSetForDownload(testContext(), token(id), false)
	}

	if len(queued) != 2 {
		t.Fatalf("queued %d items, want 2 (the new item and the item with another saved set)", len(queued))
	}
	// Without a saved set the whole set is queued, from the Download Defaults
	first := queued[0]
	if first.PosterSets[0].AutoDownload {
		t.Error("without the Auto Download setting, Auto Download should stay off")
	}
	if first.MediaItem.TMDB_ID != "1" || first.Source != models.SourceSiloPlugin || first.PosterSets[0].ID != "set" {
		t.Errorf("first queued = %+v", first)
	}
	if want := (models.SelectedTypes{Poster: true, SeasonPoster: true, Titlecard: true}); first.PosterSets[0].SelectedTypes != want {
		t.Errorf("new item types = %+v, want the Download Defaults %+v", first.PosterSets[0].SelectedTypes, want)
	}
	// With another saved set only the downloaded type is queued
	if want := (models.SelectedTypes{Titlecard: true}); queued[1].MediaItem.TMDB_ID != "2" || queued[1].PosterSets[0].SelectedTypes != want {
		t.Errorf("item with a saved set = %s %+v, want only %+v", queued[1].MediaItem.TMDB_ID, queued[1].PosterSets[0].SelectedTypes, want)
	}

	// From a refresh, a set by a creator who isn't a favorite isn't queued
	queued = nil
	refresh := token("1")
	refresh.Creator, refresh.Refresh = "Someone Else", true
	if QueueSetForDownload(testContext(), refresh, false) != 0 || len(queued) != 0 {
		t.Errorf("queued %d items from a refresh by someone who isn't a favorite, want 0", len(queued))
	}
	// From the image picker it's queued, as it was picked by hand
	refresh.Refresh = false
	if QueueSetForDownload(testContext(), refresh, true) != 1 {
		t.Error("a set picked in the image picker should be queued")
	}
	// The plugin's Auto Download setting turns Auto Download on for the set
	if !queued[len(queued)-1].PosterSets[0].AutoDownload {
		t.Error("the set should be saved with Auto Download on")
	}

}

func TestOnlyType(t *testing.T) {
	if got := onlyType("season_poster", number(0)); got != (models.SelectedTypes{SpecialSeasonPoster: true}) {
		t.Errorf("specials = %+v", got)
	}
	if got := onlyType("season_poster", number(2)); got != (models.SelectedTypes{SeasonPoster: true}) {
		t.Errorf("season 2 = %+v", got)
	}
	if !typeSelected(models.SelectedTypes{SpecialSeasonPoster: true}, "season_poster", number(0)) || typeSelected(models.SelectedTypes{SpecialSeasonPoster: true}, "season_poster", number(1)) {
		t.Error("typeSelected should tell specials from other seasons")
	}
}

func TestItemImagesForNewItem(t *testing.T) {
	useTestSecret(t, "secret")
	useFavorites(t, "first")
	useLibrary(t)
	useMovieSets(t, []models.SetRef{testSet("set", "first", models.ImageFile{ID: "poster", Type: "poster"})}, nil)
	previousCheck := lastNewItemsCheck
	t.Cleanup(func() { lastNewItemsCheck = previousCheck })
	lastNewItemsCheck = time.Time{}

	// The media server has the new item, the check adds it to aura's libraries
	checks := 0
	checkNewItems = func(context.Context) (int, logging.LogErrorInfo) {
		checks++
		cache.LibraryStore.UpdateSection(&models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: "Movies", Type: "movie"},
			MediaItems: []models.MediaItem{{TMDB_ID: "1", Type: "movie", LibraryTitle: "Movies"}}})
		return 1, logging.LogErrorInfo{}
	}

	images, Err := ItemImages(testContext(), "1", "movie", PurposeRefresh)
	if Err.Message != "" || len(images) != 1 || checks != 1 {
		t.Fatalf("new item = %d images, %d checks, %s; want 1 image after 1 check", len(images), checks, Err.Message)
	}
	// Another unknown item within 30 seconds doesn't ask the media server again
	if images, _ := ItemImages(testContext(), "2", "movie", PurposeRefresh); len(images) != 0 || checks != 1 {
		t.Errorf("second unknown item = %d images, %d checks; want none and still 1 check", len(images), checks)
	}
}
