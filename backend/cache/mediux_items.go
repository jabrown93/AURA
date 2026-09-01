package cache

import (
	"aura/models"
	"sync"
)

var MediuxItems *MediuxItemCache

type MediuxItemCache struct {
	items map[string][]*models.MediuxContentID // Key: ItemType, Value: slice of TMDBIDs
	mu    sync.RWMutex
}

func NewMediuxItemCache() *MediuxItemCache {
	return &MediuxItemCache{
		items: make(map[string][]*models.MediuxContentID),
	}
}

func init() {
	MediuxItems = NewMediuxItemCache()
}

func (c *MediuxItemCache) StoreMediuxItems(movies, shows []models.MediuxContentID) {
	items := make(map[string][]*models.MediuxContentID, 2)
	storeUnique := func(itemType string, values []models.MediuxContentID) {
		seen := make(map[string]struct{}, len(values))
		for _, value := range values {
			if value.ID == "" {
				continue
			}
			if _, exists := seen[value.ID]; exists {
				continue
			}
			seen[value.ID] = struct{}{}
			items[itemType] = append(items[itemType], &models.MediuxContentID{ID: value.ID})
		}
	}
	storeUnique("movie", movies)
	storeUnique("show", shows)

	c.mu.Lock()
	c.items = items
	c.mu.Unlock()
}

func (c *MediuxItemCache) GetMediuxItems() []models.MediuxContentID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	itemList := make([]models.MediuxContentID, 0)
	for _, items := range c.items {
		for _, item := range items {
			itemList = append(itemList, *item)
		}
	}
	return itemList
}

func (c *MediuxItemCache) GetCountMediuxItems() (movieCount int, showsCount int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	movieCount = len(c.items["movie"])
	showsCount = len(c.items["show"])
	return
}

func (c *MediuxItemCache) CheckItemExists(itemType string, tmdbID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, item := range c.items[itemType] {
		if item.ID == tmdbID {
			return true
		}
	}
	return false
}

func (c *MediuxItemCache) AddItem(itemType string, tmdbID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent duplicates
	for _, item := range c.items[itemType] {
		if item.ID == tmdbID {
			return
		}
	}
	c.items[itemType] = append(c.items[itemType], &models.MediuxContentID{
		ID: tmdbID,
	})
}

func (c *MediuxItemCache) RemoveItem(itemType string, tmdbID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	items := c.items[itemType]
	for i, item := range items {
		if item.ID == tmdbID {
			c.items[itemType] = append(items[:i], items[i+1:]...)
			break
		}
	}
}
