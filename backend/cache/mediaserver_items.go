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
	sections        map[string]*models.LibrarySection // Key: Library Title
	mu              sync.RWMutex
	generationFloor int64
	lastFullUpdate  int64
}

// NewLibraryCache creates a new LibraryCache instance
func Cache_NewLibraryCache() *MediaServerLibraryCache {
	return newLibraryCache(processArtworkVersionFloor)
}

func newLibraryCache(generationFloor int64) *MediaServerLibraryCache {
	return &MediaServerLibraryCache{
		sections:        make(map[string]*models.LibrarySection),
		generationFloor: generationFloor,
	}
}

func init() {
	LibraryStore = Cache_NewLibraryCache()
}

// UpdateSection atomically replaces one complete section snapshot.
func (c *MediaServerLibraryCache) UpdateSection(section *models.LibrarySection) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replaceSectionLocked(section)
}

// ReplaceAllSections atomically publishes a successful full refresh and prunes
// sections absent from that refresh.
func (c *MediaServerLibraryCache) ReplaceAllSections(sections []*models.LibrarySection, updatedAt int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	replacement := make(map[string]*models.LibrarySection, len(sections))
	for _, section := range sections {
		c.replaceSectionLocked(section)
		replacement[section.Title] = c.sections[section.Title]
	}
	c.sections = replacement
	c.lastFullUpdate = updatedAt
}

func (c *MediaServerLibraryCache) replaceSectionLocked(section *models.LibrarySection) {
	existingVersions := make(map[string]int64)
	if existing, found := c.sections[section.Title]; found {
		for i := range existing.MediaItems {
			key := mediaItemVersionKey(&existing.MediaItems[i])
			if key != "" && existing.MediaItems[i].UpdatedAt > existingVersions[key] {
				existingVersions[key] = existing.MediaItems[i].UpdatedAt
			}
		}
	}

	prepared := cloneLibrarySection(section)
	items := make([]models.MediaItem, 0, len(prepared.MediaItems))
	itemIndexes := make(map[string]int, len(prepared.MediaItems))
	for i := range prepared.MediaItems {
		item := &prepared.MediaItems[i]
		item.UpdatedAt = hydratedVersion(item.UpdatedAt, c.generationFloor)
		key := mediaItemVersionKey(item)
		if item.UpdatedAt < existingVersions[key] {
			item.UpdatedAt = existingVersions[key]
		}
		section.MediaItems[i].UpdatedAt = item.UpdatedAt
		if index, duplicate := itemIndexes[key]; key != "" && duplicate {
			if item.UpdatedAt < items[index].UpdatedAt {
				item.UpdatedAt = items[index].UpdatedAt
			}
			items[index] = *item
			continue
		}
		if key != "" {
			itemIndexes[key] = len(items)
		}
		items = append(items, *item)
	}
	prepared.MediaItems = items
	prepared.TotalSize = len(items)
	c.sections[prepared.Title] = prepared
}

func mediaItemVersionKey(item *models.MediaItem) string {
	if item.TMDB_ID != "" {
		return "tmdb_id:" + item.TMDB_ID
	}
	if item.RatingKey != "" {
		return "rating_key:" + item.RatingKey
	}
	return ""
}

// UpdateMediaItem updates a specific media item in a section
func (c *MediaServerLibraryCache) UpdateMediaItem(sectionTitle string, item *models.MediaItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if section exists
	if section, exists := c.sections[sectionTitle]; exists {
		existingItems := make(map[string]*models.MediaItem)
		for i := range section.MediaItems {
			key := mediaItemVersionKey(&section.MediaItems[i])
			if key != "" {
				existingItems[key] = &section.MediaItems[i]
			}
		}
		item.UpdatedAt = hydratedVersion(item.UpdatedAt, c.generationFloor)
		if existingItem, found := existingItems[mediaItemVersionKey(item)]; found {
			if item.UpdatedAt < existingItem.UpdatedAt {
				item.UpdatedAt = existingItem.UpdatedAt
			}
			*existingItem = cloneMediaItem(item)
		} else {
			section.MediaItems = append(section.MediaItems, cloneMediaItem(item))
			section.TotalSize = len(section.MediaItems)
		}
	}
}

// AdvanceMediaItemUpdatedAt advances the cached parent image version atomically.
// ratingKey is the parent key even when Plex applies artwork to a season or episode.
func (c *MediaServerLibraryCache) AdvanceMediaItemUpdatedAt(ratingKey string, now int64) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, section := range c.sections {
		for i := range section.MediaItems {
			if section.MediaItems[i].RatingKey == ratingKey {
				section.MediaItems[i].UpdatedAt = nextVersion(section.MediaItems[i].UpdatedAt, now)
				return section.MediaItems[i].UpdatedAt, true
			}
		}
	}
	return 0, false
}

func (c *MediaServerLibraryCache) SetIgnored(sectionTitle, tmdbID string, ignored bool, mode string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return false
	}
	for i := range section.MediaItems {
		if section.MediaItems[i].TMDB_ID == tmdbID {
			section.MediaItems[i].IgnoredInDB = ignored
			section.MediaItems[i].IgnoredMode = mode
			return true
		}
	}
	return false
}

func (c *MediaServerLibraryCache) UpdateMediaItemDBSavedSets(sectionTitle string, item *models.MediaItem, dbSavedSets []models.DBSavedSet) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if section exists
	if section, exists := c.sections[sectionTitle]; exists {
		// Create a map of existing items for O(1) lookup
		existingItems := make(map[string]*models.MediaItem)
		for i := range section.MediaItems {
			existingItems[section.MediaItems[i].TMDB_ID] = &section.MediaItems[i]
		}
		if existingItem, found := existingItems[item.TMDB_ID]; found {
			existingItem.DBSavedSets = append([]models.DBSavedSet(nil), dbSavedSets...)
		}
	}
}

// GetSectionByTitle retrieves a detached section copy by Title.
func (c *MediaServerLibraryCache) GetSectionByTitle(title string) (*models.LibrarySection, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	section, exists := c.sections[title]
	if !exists {
		return &models.LibrarySection{}, false
	}
	return cloneLibrarySection(section), true
}

func (c *MediaServerLibraryCache) GetLastFullUpdate() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastFullUpdate
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
		sections = append(sections, cloneLibrarySection(section))
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Title < sections[j].Title
	})

	return sections
}

// RemoveSectionByTitle removes a section from the cache by Title
func (c *MediaServerLibraryCache) RemoveSectionByTitle(title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sections, title)
}

// ClearAllSections removes all sections from the cache
func (c *MediaServerLibraryCache) ClearAllSections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sections = make(map[string]*models.LibrarySection)
}

// GetMediaItemFromSectionByTMDBID retrieves a media item by TMDB ID from a specific section
func (c *MediaServerLibraryCache) GetMediaItemFromSectionByTMDBID(sectionTitle, tmdbID string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return &models.MediaItem{}, false
	}

	for i := range section.MediaItems {
		if section.MediaItems[i].TMDB_ID == tmdbID {
			item := cloneMediaItem(&section.MediaItems[i])
			return &item, true
		}
	}
	return &models.MediaItem{}, false
}

func (c *MediaServerLibraryCache) GetMediaItemByRatingKey(ratingKey string) (*models.MediaItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, section := range c.sections {
		for i := range section.MediaItems {
			if section.MediaItems[i].RatingKey == ratingKey {
				item := cloneMediaItem(&section.MediaItems[i])
				return &item, true
			}
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
	for i := range section.MediaItems {
		item := &section.MediaItems[i]
		cleanedTitle := cleanStringForComparison(stripYearFromTitle(item.Title))
		if cleanedTitle == cleanedSearchTitle && item.Year == year {
			copy := cloneMediaItem(item)
			return &copy, true
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

	titles := make([]string, 0, len(c.sections))
	for title := range c.sections {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	var allItems []models.MediaItem
	for _, title := range titles {
		for i := range c.sections[title].MediaItems {
			allItems = append(allItems, cloneMediaItem(&c.sections[title].MediaItems[i]))
		}
	}
	return allItems
}

func cloneLibrarySection(section *models.LibrarySection) *models.LibrarySection {
	clone := *section
	clone.Paths = append([]string(nil), section.Paths...)
	clone.MediaItems = make([]models.MediaItem, len(section.MediaItems))
	for i := range section.MediaItems {
		clone.MediaItems[i] = cloneMediaItem(&section.MediaItems[i])
	}
	return &clone
}

func cloneMediaItem(item *models.MediaItem) models.MediaItem {
	clone := *item
	clone.DBSavedSets = append([]models.DBSavedSet(nil), item.DBSavedSets...)
	clone.IgnoredSets = append([]string(nil), item.IgnoredSets...)
	clone.Guids = append([]models.MediaItemGuid(nil), item.Guids...)
	if item.Movie != nil {
		movie := *item.Movie
		clone.Movie = &movie
	}
	if item.Series != nil {
		series := *item.Series
		series.Seasons = make([]models.MediaItemSeason, len(item.Series.Seasons))
		for i := range item.Series.Seasons {
			series.Seasons[i] = item.Series.Seasons[i]
			series.Seasons[i].Episodes = append([]models.MediaItemEpisode(nil), item.Series.Seasons[i].Episodes...)
		}
		clone.Series = &series
	}
	return clone
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
