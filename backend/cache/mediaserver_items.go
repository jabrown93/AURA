package cache

import (
	"aura/models"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ---   Cache Global Variables (Backend Library Cache) --- ---
var LibraryStore *MediaServerLibraryCache

type MediaServerLibraryCache struct {
	sections       map[string]*models.LibrarySection // Key: Library Title
	mu             sync.RWMutex
	LastFullUpdate int64
}

// NewLibraryCache creates a new LibraryCache instance
func Cache_NewLibraryCache() *MediaServerLibraryCache {
	return &MediaServerLibraryCache{
		sections:       make(map[string]*models.LibrarySection),
		LastFullUpdate: 0,
	}
}

func init() {
	LibraryStore = Cache_NewLibraryCache()
}

// Library caches of the other media servers, by media server ID. LibraryStore is the main media server's cache.
var (
	serverLibraryStoresMu sync.Mutex
	serverLibraryStores   = map[string]*MediaServerLibraryCache{}
)

// ServerLibraryStore returns the library cache of a media server that isn't the main one
func ServerLibraryStore(serverID string) *MediaServerLibraryCache {
	serverLibraryStoresMu.Lock()
	defer serverLibraryStoresMu.Unlock()
	store, found := serverLibraryStores[serverID]
	if !found {
		store = Cache_NewLibraryCache()
		serverLibraryStores[serverID] = store
	}
	return store
}

// MediaServerLibraryStore returns the library cache of any media server
func MediaServerLibraryStore(serverID string, isMain bool) *MediaServerLibraryCache {
	if isMain {
		return LibraryStore
	}
	return ServerLibraryStore(serverID)
}

// KeepServerLibraryStores removes the library caches of media servers that are no longer used
func KeepServerLibraryStores(serverIDs []string) {
	serverLibraryStoresMu.Lock()
	defer serverLibraryStoresMu.Unlock()
	keep := map[string]bool{}
	for _, id := range serverIDs {
		keep[id] = true
	}
	for id := range serverLibraryStores {
		if !keep[id] {
			delete(serverLibraryStores, id)
		}
	}
}

// UpdateSection updates or adds a LibrarySection in the cache.
// If the section already exists, its metadata and media items are updated.
// New media items are appended to the section.
// If the section does not exist, it is added to the cache.
func (c *MediaServerLibraryCache) UpdateSection(section *models.LibrarySection) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if section already exists
	// If it does, we need to update it
	if existing, exists := c.sections[section.Title]; exists {
		// Update metadata
		existing.Type = section.Type
		existing.ID = section.ID
		existing.Paths = section.Paths

		// Create a map of existing items for O(1) lookup.
		// Keyed by TMDB_ID+Edition so multiple editions of the same TMDB item
		// (e.g. Theatrical vs Director's Cut) don't collapse into one entry.
		existingItems := make(map[string]*models.MediaItem)
		for i := range existing.MediaItems {
			existingItems[mediaItemCacheKey(&existing.MediaItems[i])] = &existing.MediaItems[i]
		}

		// Update existing items and collect new ones
		var newItems []models.MediaItem
		for _, newItem := range section.MediaItems {
			if existingItem, found := existingItems[mediaItemCacheKey(&newItem)]; found {
				// Update existing item
				*existingItem = newItem
			} else {
				// Collect new item for appending
				newItems = append(newItems, newItem)
			}
		}

		// Append new items
		existing.MediaItems = append(existing.MediaItems, newItems...)
		existing.TotalSize = len(existing.MediaItems)
	} else {
		// If section does not exist, add it to the cache
		c.sections[section.Title] = section
	}
}

// UpdateMediaItem updates a specific media item in a section
func (c *MediaServerLibraryCache) UpdateMediaItem(sectionTitle string, item *models.MediaItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if section exists
	if section, exists := c.sections[sectionTitle]; exists {
		// Find the item directly instead of building a lookup of every item each time
		for i := range section.MediaItems {
			if section.MediaItems[i].TMDB_ID == item.TMDB_ID && section.MediaItems[i].Edition == item.Edition {
				// Update existing item
				section.MediaItems[i] = *item
				return
			}
		}
		// Append new item
		section.MediaItems = append(section.MediaItems, *item)
		section.TotalSize = len(section.MediaItems)
	}
}

// UpdateMediaItemDBState sets an item's saved sets and ignored state, as they are in the database
func (c *MediaServerLibraryCache) UpdateMediaItemDBState(sectionTitle, tmdbID, edition string, sets []models.DBSavedSet, ignored bool, ignoredMode string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return
	}
	for i := range section.MediaItems {
		item := &section.MediaItems[i]
		if item.TMDB_ID == tmdbID && item.Edition == edition {
			item.DBSavedSets = sets
			item.IgnoredInDB = ignored
			item.IgnoredMode = ignoredMode
		}
	}
}

// RemoveItemsNotSeen removes the items of a section that a full load of it didn't see
func (c *MediaServerLibraryCache) RemoveItemsNotSeen(sectionTitle string, seen map[string]bool) (removed int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return 0
	}
	kept := section.MediaItems[:0]
	for _, item := range section.MediaItems {
		if seen[mediaItemCacheKey(&item)] {
			kept = append(kept, item)
		}
	}
	removed = len(section.MediaItems) - len(kept)
	section.MediaItems = kept
	section.TotalSize = len(kept)
	return removed
}

// SectionItemKeys returns the cache keys of a section's items
func (c *MediaServerLibraryCache) SectionItemKeys(sectionTitle string) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := map[string]bool{}
	if section, exists := c.sections[sectionTitle]; exists {
		for i := range section.MediaItems {
			keys[mediaItemCacheKey(&section.MediaItems[i])] = true
		}
	}
	return keys
}

// SectionItemCount returns how many items a section has
func (c *MediaServerLibraryCache) SectionItemCount(sectionTitle string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if section, exists := c.sections[sectionTitle]; exists {
		return len(section.MediaItems)
	}
	return 0
}

// MediaItemCacheKey is how the cache tells items apart
func MediaItemCacheKey(item *models.MediaItem) string {
	return mediaItemCacheKey(item)
}

// GetSectionByTitle retrieves a section by Title
func (c *MediaServerLibraryCache) GetSectionByTitle(title string) (*models.LibrarySection, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	section, exists := c.sections[title]
	return section, exists
}

func (c *MediaServerLibraryCache) GetRatingKeyByTMDBID(libraryTitle, tmdbID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[libraryTitle]
	if !exists {
		return "", false
	}

	for _, item := range section.MediaItems {
		if item.TMDB_ID == tmdbID {
			return item.RatingKey, true
		}
	}

	return "", false
}

// GetAllSectionsSortedByTitle returns all sections sorted by Title
func (c *MediaServerLibraryCache) GetAllSectionsSortedByTitle() []*models.LibrarySection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sections := make([]*models.LibrarySection, 0, len(c.sections))
	for _, section := range c.sections {
		sections = append(sections, section)
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Title < sections[j].Title
	})

	return sections
}

// ForEachMediaItem calls fn for every item, sections in title order. fn must not change the cache
func (c *MediaServerLibraryCache) ForEachMediaItem(fn func(section models.LibrarySectionBase, item *models.MediaItem)) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	titles := make([]string, 0, len(c.sections))
	for title := range c.sections {
		titles = append(titles, title)
	}
	sort.Strings(titles)
	for _, title := range titles {
		section := c.sections[title]
		for i := range section.MediaItems {
			fn(section.LibrarySectionBase, &section.MediaItems[i])
		}
	}
}

// ClearAllSections removes all sections from the cache
func (c *MediaServerLibraryCache) ClearAllSections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sections = make(map[string]*models.LibrarySection)
}

// mediaItemCacheKey uniquely identifies a media item within a section cache.
// Edition is included so multiple editions of the same TMDB item (e.g.
// Theatrical vs Director's Cut) are tracked as distinct cache entries.
func mediaItemCacheKey(item *models.MediaItem) string {
	return item.TMDB_ID + "|" + item.Edition
}

// GetMediaItemFromSectionByTMDBID retrieves a media item by TMDB ID from a specific section.
// If multiple editions exist for the TMDB ID, an arbitrary one is returned -
// callers that need a specific edition should use GetMediaItemFromSectionByTMDBIDAndEdition.
func (c *MediaServerLibraryCache) GetMediaItemFromSectionByTMDBID(sectionTitle, tmdbID string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return &models.MediaItem{}, false
	}

	var newestItem *models.MediaItem
	for _, item := range section.MediaItems {
		if item.TMDB_ID == tmdbID {
			newestItem = &item
			break
		}
	}

	if newestItem != nil {
		return newestItem, true
	}
	return &models.MediaItem{}, false
}

// GetMediaItemFromSectionByTMDBIDAndEdition retrieves a specific edition of a
// media item by TMDB ID from a specific section.
func (c *MediaServerLibraryCache) GetMediaItemFromSectionByTMDBIDAndEdition(sectionTitle, tmdbID, edition string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return &models.MediaItem{}, false
	}

	for _, item := range section.MediaItems {
		if item.TMDB_ID == tmdbID && item.Edition == edition {
			return &item, true
		}
	}

	return &models.MediaItem{}, false
}

// GetMediaItemsByTMDBID returns the items with the TMDB ID in every library
func (c *MediaServerLibraryCache) GetMediaItemsByTMDBID(tmdbID string) []models.MediaItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := []models.MediaItem{}
	if tmdbID == "" {
		return items
	}
	for _, section := range c.sections {
		for _, item := range section.MediaItems {
			if item.TMDB_ID == tmdbID {
				items = append(items, item)
			}
		}
	}
	return items
}

func (c *MediaServerLibraryCache) GetMediaItemByRatingKey(ratingKey string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, section := range c.sections {
		for _, item := range section.MediaItems {
			if item.RatingKey == ratingKey {
				return &item, true
			}
		}
	}

	return &models.MediaItem{}, false
}

// GetMediaItemsByRatingKey retrieves the media item with this rating key from every section it is in.
// Some media servers (e.g. Silo) use the same rating key for an item in every library it is in.
func (c *MediaServerLibraryCache) GetMediaItemsByRatingKey(ratingKey string) []models.MediaItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := []models.MediaItem{}
	for _, section := range c.sections {
		for _, item := range section.MediaItems {
			if item.RatingKey == ratingKey {
				items = append(items, item)
			}
		}
	}
	return items
}

// GetMediaItemFromSectionByRatingKey retrieves a media item by rating key from a specific section.
// Some media servers (e.g. Silo) use the same rating key for an item in every library it is in.
func (c *MediaServerLibraryCache) GetMediaItemFromSectionByRatingKey(sectionTitle, ratingKey string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return &models.MediaItem{}, false
	}

	for _, item := range section.MediaItems {
		if item.RatingKey == ratingKey {
			return &item, true
		}
	}

	return &models.MediaItem{}, false
}

// GetMediaItemFromSectionByTitleAndYear retrieves the TMDB ID from a media item by its title and year
func (c *MediaServerLibraryCache) GetMediaItemFromSectionByTitleAndYear(sectionTitle, itemTitle string, year int) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return &models.MediaItem{}, false
	}

	cleanedSearchTitle := cleanStringForComparison(stripYearFromTitle(itemTitle))
	for _, item := range section.MediaItems {
		cleanedTitle := cleanStringForComparison(stripYearFromTitle(item.Title))
		if cleanedTitle == cleanedSearchTitle && item.Year == year {
			return &item, true
		}
	}

	return &models.MediaItem{}, false
}

func (c *MediaServerLibraryCache) GetSectionsCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sections)
}

func (c *MediaServerLibraryCache) GetItemsCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	totalItems := 0
	for _, section := range c.sections {
		totalItems += len(section.MediaItems)
	}
	return totalItems
}

func (c *MediaServerLibraryCache) GetAllMediaItems() []models.MediaItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var allItems []models.MediaItem
	for _, section := range c.sections {
		allItems = append(allItems, section.MediaItems...)
	}
	return allItems
}

// IsEmpty checks if the cache is empty
func (c *MediaServerLibraryCache) IsEmpty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sections) == 0
}

func cleanStringForComparison(input string) string {
	var b strings.Builder
	input = strings.ToLower(input)
	for _, r := range input {
		switch r {
		case '-', '_', '.', ',', ':', ';', '!', '?', '\'', '(', ')', '[', ']', '{', '}':
			// skip these characters
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func stripYearFromTitle(title string) string {
	parts := strings.Fields(title)
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if _, err := strconv.Atoi(last); err == nil {
			return strings.Join(parts[:len(parts)-1], " ")
		}
	}
	return title
}
