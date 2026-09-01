package cache

import (
	"aura/models"
	"sync"
)

// ---   Cache Global Variables (Backend Library Collections Cache) --- ---
var CollectionsStore *MediaServerCollectionsCache

type MediaServerCollectionsCache struct {
	// libraryTitle -> index -> CollectionItem
	collections     map[string]map[string]*models.CollectionItem
	mu              sync.RWMutex
	generationFloor int64
	LastFullUpdate  int64
}

// NewCollectionsCache creates a new CollectionsCache instance
func Cache_NewCollectionsCache() *MediaServerCollectionsCache {
	return newCollectionsCache(processArtworkVersionFloor)
}

func newCollectionsCache(generationFloor int64) *MediaServerCollectionsCache {
	return &MediaServerCollectionsCache{
		collections:     make(map[string]map[string]*models.CollectionItem),
		generationFloor: generationFloor,
		LastFullUpdate:  0,
	}
}

func init() {
	CollectionsStore = Cache_NewCollectionsCache()
}

func (msc *MediaServerCollectionsCache) GetAllCollections() []models.CollectionItem {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	out := []models.CollectionItem{}
	for _, lib := range msc.collections {
		for _, coll := range lib {
			out = append(out, *coll)
		}
	}
	return out
}

func (msc *MediaServerCollectionsCache) GetCollectionLibraryTitles() []string {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	titles := []string{}
	for title := range msc.collections {
		titles = append(titles, title)
	}
	return titles
}

func (msc *MediaServerCollectionsCache) GetTotalCollectionsCount() int {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	total := 0
	for _, lib := range msc.collections {
		total += len(lib)
	}
	return total
}

func (msc *MediaServerCollectionsCache) GetCollection(libraryTitle string, index string) (*models.CollectionItem, bool) {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	lib := msc.collections[libraryTitle]
	if lib == nil {
		return nil, false
	}

	coll, ok := lib[index]
	return coll, ok
}

func (msc *MediaServerCollectionsCache) GetCollectionByRatingKey(ratingKey string) (*models.CollectionItem, bool) {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	for _, lib := range msc.collections {
		for _, coll := range lib {
			if coll.RatingKey == ratingKey {
				copy := *coll
				return &copy, true
			}
		}
	}
	return nil, false
}

// AdvanceCollectionArtworkVersion advances a cached collection image version atomically.
func (msc *MediaServerCollectionsCache) AdvanceCollectionArtworkVersion(ratingKey string, now int64) (int64, bool) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	for _, lib := range msc.collections {
		for _, collection := range lib {
			if collection.RatingKey == ratingKey {
				collection.ArtworkVersion = nextVersion(collection.ArtworkVersion, now)
				return collection.ArtworkVersion, true
			}
		}
	}
	return 0, false
}

func (msc *MediaServerCollectionsCache) GetCollectionsByLibrary(libraryTitle string) []models.CollectionItem {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	out := []models.CollectionItem{}
	lib := msc.collections[libraryTitle]
	for _, coll := range lib {
		out = append(out, *coll)
	}
	return out
}

func (msc *MediaServerCollectionsCache) UpsertCollection(collection *models.CollectionItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	lib := msc.collections[collection.LibraryTitle]
	if lib == nil {
		lib = make(map[string]*models.CollectionItem)
		msc.collections[collection.LibraryTitle] = lib
	}
	collection.ArtworkVersion = hydratedVersion(max(collection.UpdatedAt, collection.ArtworkVersion), msc.generationFloor)
	if existing := lib[collection.RatingKey]; existing != nil {
		collection.UpdatedAt = max(collection.UpdatedAt, existing.UpdatedAt)
		collection.ArtworkVersion = max(collection.ArtworkVersion, existing.ArtworkVersion)
	}
	lib[collection.RatingKey] = collection
}

func (msc *MediaServerCollectionsCache) UpdateMediaItemInCollectionByIndex(collectionIndex string, item *models.MediaItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	if item == nil || item.LibraryTitle == "" {
		return
	}

	lib := msc.collections[item.LibraryTitle]
	if lib == nil {
		return
	}

	collection, ok := lib[collectionIndex]
	if !ok || collection == nil {
		return
	}

	for i := range collection.MediaItems {
		if collection.MediaItems[i].RatingKey == item.RatingKey {
			// Update existing item
			collection.MediaItems[i] = *item
			return
		}
	}

	// If not found, append new item
	collection.MediaItems = append(collection.MediaItems, *item)
}

func (msc *MediaServerCollectionsCache) UpdateMediaItemInCollectionByTitle(collectionTitle string, item *models.MediaItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	if item == nil || item.LibraryTitle == "" {
		return
	}

	lib := msc.collections[item.LibraryTitle]
	if lib == nil {
		return
	}

	for _, collection := range lib {
		if collection == nil || collection.Title != collectionTitle {
			continue
		}

		for i := range collection.MediaItems {
			if collection.MediaItems[i].RatingKey == item.RatingKey {
				collection.MediaItems[i] = *item
				return
			}
		}

		collection.MediaItems = append(collection.MediaItems, *item)
		return
	}

}
