package cache

import (
	"aura/models"
	"sync"
	"testing"
)

func TestHydrationPreservesUpdatedAtAndFloorsArtworkVersion(t *testing.T) {
	library := newLibraryCache(500)
	section := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{RatingKey: "older", UpdatedAt: 100},
			{RatingKey: "newer", UpdatedAt: 200},
		},
	}

	library.UpdateSection(section)

	if section.MediaItems[0].UpdatedAt != 100 || section.MediaItems[1].UpdatedAt != 200 {
		t.Fatalf("updated_at values flattened: %+v", section.MediaItems)
	}
	for _, item := range section.MediaItems {
		if item.ArtworkVersion != 500 {
			t.Fatalf("%s artwork version = %d, want generation floor 500", item.RatingKey, item.ArtworkVersion)
		}
	}

	collections := newCollectionsCache(500)
	collection := &models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: 100}
	collections.UpsertCollection(collection)
	if collection.UpdatedAt != 100 || collection.ArtworkVersion != 500 {
		t.Fatalf("collection = %+v, want updated_at 100 and artwork_version 500", collection)
	}
}

func TestArtworkVersionsAdvanceMonotonically(t *testing.T) {
	library := newLibraryCache(0)
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 100}},
	})
	collections := newCollectionsCache(0)
	collections.UpsertCollection(&models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", UpdatedAt: 100})

	if got, ok := library.AdvanceMediaItemArtworkVersion("show-1", 100); !ok || got != 101 {
		t.Fatalf("media version = %d, found = %v; want 101, true", got, ok)
	}
	if got, ok := library.AdvanceMediaItemArtworkVersion("show-1", 100); !ok || got != 102 {
		t.Fatalf("same-second media version = %d, found = %v; want 102, true", got, ok)
	}
	if got, ok := collections.AdvanceCollectionArtworkVersion("collection-1", 100); !ok || got != 101 {
		t.Fatalf("collection version = %d, found = %v; want 101, true", got, ok)
	}

	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 100}},
	})
	collections.UpsertCollection(&models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "TV", UpdatedAt: 100})
	if item, _ := library.GetMediaItemByRatingKey("show-1"); item.UpdatedAt != 100 || item.ArtworkVersion != 102 {
		t.Fatalf("library refresh changed timestamp or regressed version: %+v", item)
	}
	if collection, _ := collections.GetCollectionByRatingKey("collection-1"); collection.UpdatedAt != 100 || collection.ArtworkVersion != 101 {
		t.Fatalf("collection refresh changed timestamp or regressed version: %+v", collection)
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
			library.AdvanceMediaItemArtworkVersion("movie-1", 100)
		}()
	}
	wg.Wait()

	item, ok := library.GetMediaItemByRatingKey("movie-1")
	if !ok {
		t.Fatal("movie-1 missing from cache")
	}
	if item.UpdatedAt != 100 || item.ArtworkVersion != 100+applies {
		t.Fatalf("media item = %+v, want timestamp 100 and version %d", item, 100+applies)
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
	library.AdvanceMediaItemArtworkVersion("section-item", 700)
	library.AdvanceMediaItemArtworkVersion("detail-item", 700)

	browseResult := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems: []models.MediaItem{
			{RatingKey: "section-item", UpdatedAt: 100},
			{RatingKey: "detail-item", UpdatedAt: 100},
		},
	}
	library.UpdateSection(browseResult)
	if got := browseResult.MediaItems[0]; got.UpdatedAt != 600 || got.ArtworkVersion != 700 {
		t.Fatalf("browse result = %+v, want timestamp 600 and version 700", got)
	}

	detailResult := &models.MediaItem{RatingKey: "detail-item", UpdatedAt: 100}
	library.UpdateMediaItem("Movies", detailResult)
	if detailResult.UpdatedAt != 600 || detailResult.ArtworkVersion != 700 {
		t.Fatalf("detail result = %+v, want timestamp 600 and version 700", detailResult)
	}

	collections := newCollectionsCache(500)
	collection := &models.CollectionItem{RatingKey: "collection-1", LibraryTitle: "Movies", UpdatedAt: 600}
	collections.UpsertCollection(collection)
	collections.AdvanceCollectionArtworkVersion(collection.RatingKey, 700)
	collectionResult := &models.CollectionItem{RatingKey: collection.RatingKey, LibraryTitle: "Movies", UpdatedAt: 100}
	collections.UpsertCollection(collectionResult)
	if collectionResult.UpdatedAt != 600 || collectionResult.ArtworkVersion != 700 {
		t.Fatalf("collection result = %+v, want timestamp 600 and version 700", collectionResult)
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
	if firstResult.UpdatedAt != 100 || firstResult.ArtworkVersion != 1000 {
		t.Fatalf("first process result = %+v, want timestamp 100 and version 1000", firstResult)
	}

	repeatedResult := &models.MediaItem{RatingKey: "movie-1", UpdatedAt: 100}
	firstProcess.UpdateMediaItem("Movies", repeatedResult)
	if repeatedResult.UpdatedAt != firstResult.UpdatedAt || repeatedResult.ArtworkVersion != firstResult.ArtworkVersion {
		t.Fatalf("same-process results differ: %+v and %+v", firstResult, repeatedResult)
	}

	restartedProcess := newLibraryCache(1001)
	restartedSection := &models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "Movies"},
		MediaItems:         []models.MediaItem{{RatingKey: "movie-1", UpdatedAt: 100}},
	}
	restartedProcess.UpdateSection(restartedSection)
	if got := restartedSection.MediaItems[0]; got.UpdatedAt != 100 || got.ArtworkVersion != 1001 {
		t.Fatalf("restarted process result = %+v, want timestamp 100 and version 1001", got)
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
