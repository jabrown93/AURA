package cache

import (
	"aura/models"
	"sync"
	"testing"
)

func TestArtworkVersionsAdvanceMonotonically(t *testing.T) {
	library := Cache_NewLibraryCache()
	library.UpdateSection(&models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{Title: "TV"},
		MediaItems:         []models.MediaItem{{RatingKey: "show-1", UpdatedAt: 100}},
	})
	collections := Cache_NewCollectionsCache()
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
	library := Cache_NewLibraryCache()
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
