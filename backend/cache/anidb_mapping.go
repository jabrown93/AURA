package cache

import (
	"aura/models"
	"sync"
	"time"
)

var AnidbMappings *AnidbMappingCache

type AnidbMappingCache struct {
	mappings       map[string]*models.AnidbMapping // Key: AniDB ID
	mu             sync.RWMutex
	LastFullUpdate int64
}

func Cache_NewAnidbMappingCache() *AnidbMappingCache {
	return &AnidbMappingCache{
		mappings: make(map[string]*models.AnidbMapping),
	}
}

func init() {
	AnidbMappings = Cache_NewAnidbMappingCache()
}

// StoreAnidbMappings replaces the entire cache with a fresh dataset. The map is
// swapped wholesale (rather than merged) so stale AniDB entries dropped from an
// updated dataset don't linger.
func (c *AnidbMappingCache) StoreAnidbMappings(mappings []models.AnidbMapping) {
	next := make(map[string]*models.AnidbMapping, len(mappings))
	for i := range mappings {
		m := &mappings[i]
		if m.AnidbID == "" {
			continue
		}
		next[m.AnidbID] = m
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.mappings = next
	c.LastFullUpdate = time.Now().Unix()
}

func (c *AnidbMappingCache) GetByAnidbID(anidbID string) (*models.AnidbMapping, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m, ok := c.mappings[anidbID]
	return m, ok
}

func (c *AnidbMappingCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.mappings)
}
