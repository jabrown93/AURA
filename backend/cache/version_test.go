package cache

import (
	"aura/models"
	"sync"
	"testing"
)

func TestArtworkVersionsAdvanceMonotonically(t *testing.T) {
	library := newLibraryCache(0)
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 100}},
	})
	collections := newCollectionsCache(0)
	collections.UpsertCollection(&models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", UpdatedAt: 100})

	if got, ok := library.AdvanceMediaItemUpdatedAt("show-1", 100); !ok || got != 101 {
		t.Fatalf("media version = %d, found = %v; want 101, true", got, ok)
	}
	if got, ok := library.AdvanceMediaItemUpdatedAt("show-1", 100); !ok || got != 102 {
		t.Fatalf("same-second media version = %d, found = %v; want 102, true", got, ok)
	}
	if got, ok := collections.AdvanceCollectionUpdatedAt("collection-1", 100); !ok || got != 101 {
		t.Fatalf("collection version = %d, found = %v; want 101, true", got, ok)
	}

	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 100}},
	})
	collections.UpsertCollection(&models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", UpdatedAt: 100})
	if item, _ := library.GetMediaItemByRatingKey("show-1"); item.UpdatedAt != 102 {
		t.Fatalf("library refresh regressed version to %d", item.UpdatedAt)
	}
	if collection, _ := collections.GetCollectionByRatingKey("collection-1"); collection.UpdatedAt != 101 {
		t.Fatalf("collection refresh regressed version to %d", collection.UpdatedAt)
	}
}

func TestConcurrentArtworkVersionAdvance(t *testing.T) {
	const applies = 32
	library := newLibraryCache(0)
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "movie-1", UpdatedAt: 100}},
	})

	var wg sync.WaitGroup
	wg.Add(applies)
	for range applies {
		go func() {
			defer wg.Done()
			library.AdvanceMediaItemUpdatedAt("movie-1", 100)
		}()
	}
	wg.Wait()

	item, ok := library.GetMediaItemByRatingKey("movie-1")
	if !ok {
		t.Fatal("movie-1 missing from cache")
	}
	if item.UpdatedAt != 100+applies {
		t.Fatalf("media version = %d, want %d", item.UpdatedAt, 100+applies)
	}
}

func TestHydrationWritesMonotonicVersionsBackToResults(t *testing.T) {
	library := newLibraryCache(500)
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{RatingKey: "section-item", UpdatedAt: 600},
			{RatingKey: "detail-item", UpdatedAt: 600},
		},
	})
	library.AdvanceMediaItemUpdatedAt("section-item", 700)
	library.AdvanceMediaItemUpdatedAt("detail-item", 700)

	browseResult := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{RatingKey: "section-item", UpdatedAt: 100},
			{RatingKey: "detail-item", UpdatedAt: 100},
		},
	}
	library.UpdateSection(browseResult)
	if got := browseResult.MediaItems[0].UpdatedAt; got != 700 {
		t.Fatalf("browse result version = %d, want advanced cache version 700", got)
	}

	detailResult := &models.MediaItem{RatingKey: "detail-item", UpdatedAt: 100}
	library.UpdateMediaItem("Movies", detailResult)
	if detailResult.UpdatedAt != 700 {
		t.Fatalf("detail result version = %d, want advanced cache version 700", detailResult.UpdatedAt)
	}

	collections := newCollectionsCache(500)
	collection := &models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: 600}
	collections.UpsertCollection(collection)
	collections.AdvanceCollectionUpdatedAt(collection.RatingKey, 700)
	collectionResult := &models.CollectionItem{RatingKey: collection.RatingKey, LibraryTitle: "Movies", UpdatedAt: 100}
	collections.UpsertCollection(collectionResult)
	if collectionResult.UpdatedAt != 700 {
		t.Fatalf("collection result version = %d, want advanced cache version 700", collectionResult.UpdatedAt)
	}
}

func TestHydrationUsesStableProcessGenerationAndChangesAfterRestart(t *testing.T) {
	firstProcess := newLibraryCache(1000)
	firstResult := &models.MediaItem{RatingKey: "movie-1", UpdatedAt: 100}
	firstProcess.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{*firstResult},
	})
	firstProcess.UpdateMediaItem("Movies", firstResult)
	if firstResult.UpdatedAt != 1000 {
		t.Fatalf("first process version = %d, want generation floor 1000", firstResult.UpdatedAt)
	}

	repeatedResult := &models.MediaItem{RatingKey: "movie-1", UpdatedAt: 100}
	firstProcess.UpdateMediaItem("Movies", repeatedResult)
	if repeatedResult.UpdatedAt != firstResult.UpdatedAt {
		t.Fatalf("same-process versions differ: %d and %d", firstResult.UpdatedAt, repeatedResult.UpdatedAt)
	}

	restartedProcess := newLibraryCache(1001)
	restartedSection := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "movie-1", UpdatedAt: 100}},
	}
	restartedProcess.UpdateSection(restartedSection)
	if got := restartedSection.MediaItems[0].UpdatedAt; got != 1001 {
		t.Fatalf("restarted process version = %d, want new generation floor 1001", got)
	}

	if Cache_NewLibraryCache().generationFloor != processArtworkVersionFloor || Cache_NewCollectionsCache().generationFloor != processArtworkVersionFloor {
		t.Fatal("public caches do not share stable process generation floor")
	}
}

func TestVersionOwnershipUsesRatingKeyWhenTMDBIDIsEmpty(t *testing.T) {
	library := newLibraryCache(0)
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems: []models.MediaItem{
			{RatingKey: "show-1", Title: "One", UpdatedAt: 100},
			{RatingKey: "show-2", Title: "Two", UpdatedAt: 200},
		},
	})

	browseResult := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems: []models.MediaItem{
			{RatingKey: "show-1", Title: "One refreshed", UpdatedAt: 101},
			{RatingKey: "show-2", Title: "Two", UpdatedAt: 200},
		},
	}
	library.UpdateSection(browseResult)

	one, _ := library.GetMediaItemByRatingKey("show-1")
	two, _ := library.GetMediaItemByRatingKey("show-2")
	if one.Title != "One refreshed" || one.UpdatedAt != 101 {
		t.Fatalf("show-1 = %+v, want independently refreshed item", one)
	}
	if two.Title != "Two" || two.UpdatedAt != 200 {
		t.Fatalf("show-2 disturbed by empty TMDB ID collision: %+v", two)
	}
}
