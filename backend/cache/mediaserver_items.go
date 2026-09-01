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
	sections               map[string]*models.LibrarySection // Key: Library Title
	mu                     sync.RWMutex
	generationFloor        int64
	dbMutationGeneration   uint64
	itemMutationGeneration map[mediaItemMutationKey]uint64
	lastFullUpdate         int64
}

type mediaItemMutationKey struct {
	sectionTitle string
	ratingKey    string
	fallbackKey  string
}

// NewLibraryCache creates a new LibraryCache instance
func Cache_NewLibraryCache() *MediaServerLibraryCache {
	return newLibraryCache(processArtworkVersionFloor)
}

func newLibraryCache(generationFloor int64) *MediaServerLibraryCache {
	return &MediaServerLibraryCache{
		sections:               make(map[string]*models.LibrarySection),
		generationFloor:        generationFloor,
		itemMutationGeneration: make(map[mediaItemMutationKey]uint64),
	}
}

func init() {
	LibraryStore = Cache_NewLibraryCache()
}

// DBMutationGeneration captures cache mutations to database-owned item fields.
func (c *MediaServerLibraryCache) DBMutationGeneration() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dbMutationGeneration
}

// UpdateSection atomically replaces one complete authoritative snapshot.
func (c *MediaServerLibraryCache) UpdateSection(section *models.LibrarySection) {
	c.UpdateSectionFromDBSnapshot(section, c.DBMutationGeneration())
}

// UpdateSectionFromDBSnapshot preserves only database-owned fields mutated after
// snapshotGeneration was captured.
func (c *MediaServerLibraryCache) UpdateSectionFromDBSnapshot(section *models.LibrarySection, snapshotGeneration uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replaceSectionLocked(section, snapshotGeneration)
}

// ReplaceAllSections atomically publishes a successful authoritative refresh.
func (c *MediaServerLibraryCache) ReplaceAllSections(sections []*models.LibrarySection, updatedAt int64) {
	c.ReplaceAllSectionsFromDBSnapshot(sections, updatedAt, c.DBMutationGeneration())
}

// ReplaceAllSectionsFromDBSnapshot publishes a successful full refresh and
// prunes sections absent from that refresh.
func (c *MediaServerLibraryCache) ReplaceAllSectionsFromDBSnapshot(sections []*models.LibrarySection, updatedAt int64, snapshotGeneration uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	replacement := make(map[string]*models.LibrarySection, len(sections))
	retainedGenerations := make(map[mediaItemMutationKey]uint64)
	for _, section := range sections {
		c.replaceSectionLocked(section, snapshotGeneration)
		replacement[section.Title] = c.sections[section.Title]
		for key, generation := range c.itemMutationGeneration {
			if key.sectionTitle == section.Title {
				retainedGenerations[key] = generation
			}
		}
	}
	c.sections = replacement
	c.itemMutationGeneration = retainedGenerations
	c.lastFullUpdate = updatedAt
}

func (c *MediaServerLibraryCache) replaceSectionLocked(section *models.LibrarySection, snapshotGeneration uint64) {
	prepared := cloneLibrarySection(section)
	var existingItems []models.MediaItem
	if existing, found := c.sections[section.Title]; found {
		existingItems = existing.MediaItems
	}

	existingByRatingKey := make(map[string]*models.MediaItem, len(existingItems))
	existingFallbacks := make(map[string][]*models.MediaItem, len(existingItems))
	for i := range existingItems {
		item := &existingItems[i]
		if item.RatingKey != "" {
			existingByRatingKey[item.RatingKey] = item
		}
		if key := mediaItemFallbackKey(item); key != "" {
			existingFallbacks[key] = append(existingFallbacks[key], item)
		}
	}
	preparedFallbackCounts := make(map[string]int, len(prepared.MediaItems))
	for i := range prepared.MediaItems {
		if key := mediaItemFallbackKey(&prepared.MediaItems[i]); key != "" {
			preparedFallbackCounts[key]++
		}
	}

	existingGenerations := make(map[mediaItemMutationKey]uint64)
	for key, generation := range c.itemMutationGeneration {
		if key.sectionTitle == section.Title {
			existingGenerations[key] = generation
			delete(c.itemMutationGeneration, key)
		}
	}
	items := make([]models.MediaItem, 0, len(prepared.MediaItems))
	itemIndexes := make(map[string]int, len(prepared.MediaItems))
	for i := range prepared.MediaItems {
		item := &prepared.MediaItems[i]
		item.UpdatedAt = hydratedVersion(item.UpdatedAt, c.generationFloor)
		existing := existingByRatingKey[item.RatingKey]
		if existing == nil {
			key := mediaItemFallbackKey(item)
			if candidates := existingFallbacks[key]; key != "" && len(candidates) == 1 && preparedFallbackCounts[key] == 1 {
				existing = candidates[0]
			}
		}
		if existing != nil {
			generation := existingGenerations[mediaItemMutationKeyFor(section.Title, existing)]
			if generation > snapshotGeneration {
				preserveDBOwnedMediaItemState(item, existing)
				c.itemMutationGeneration[mediaItemMutationKeyFor(section.Title, item)] = generation
			}
			preserveMediaItemVersion(item, existing)
		}
		section.MediaItems[i].UpdatedAt = item.UpdatedAt
		if index, duplicate := itemIndexes[item.RatingKey]; item.RatingKey != "" && duplicate {
			if item.UpdatedAt < items[index].UpdatedAt {
				item.UpdatedAt = items[index].UpdatedAt
			}
			items[index] = *item
			continue
		}
		if item.RatingKey != "" {
			itemIndexes[item.RatingKey] = len(items)
		}
		items = append(items, *item)
	}
	prepared.MediaItems = items
	prepared.TotalSize = len(items)
	c.sections[prepared.Title] = prepared
}

func mediaItemFallbackKey(item *models.MediaItem) string {
	if item.TMDB_ID == "" {
		return ""
	}
	return item.Type + "\x00" + item.TMDB_ID
}

func mediaItemMutationKeyFor(sectionTitle string, item *models.MediaItem) mediaItemMutationKey {
	key := mediaItemMutationKey{sectionTitle: sectionTitle, ratingKey: item.RatingKey}
	if item.RatingKey == "" {
		key.fallbackKey = mediaItemFallbackKey(item)
	}
	return key
}

func preserveDBOwnedMediaItemState(item, existing *models.MediaItem) {
	item.DBSavedSets = append([]models.DBSavedSet(nil), existing.DBSavedSets...)
	item.IgnoredInDB = existing.IgnoredInDB
	item.IgnoredMode = existing.IgnoredMode
	item.IgnoredSets = append([]string(nil), existing.IgnoredSets...)
}

func preserveMediaItemVersion(item, existing *models.MediaItem) {
	if item.UpdatedAt < existing.UpdatedAt {
		item.UpdatedAt = existing.UpdatedAt
	}
}

// UpdateMediaItem updates a specific media item in a section
func (c *MediaServerLibraryCache) UpdateMediaItem(sectionTitle string, item *models.MediaItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if section, exists := c.sections[sectionTitle]; exists {
		var existingItem *models.MediaItem
		if item.RatingKey != "" {
			for i := range section.MediaItems {
				if section.MediaItems[i].RatingKey == item.RatingKey {
					existingItem = &section.MediaItems[i]
					break
				}
			}
		}
		if existingItem == nil {
			key := mediaItemFallbackKey(item)
			for i := range section.MediaItems {
				if key != "" && mediaItemFallbackKey(&section.MediaItems[i]) == key {
					if existingItem != nil {
						existingItem = nil
						break
					}
					existingItem = &section.MediaItems[i]
				}
			}
		}
		item.UpdatedAt = hydratedVersion(item.UpdatedAt, c.generationFloor)
		if existingItem != nil {
			oldKey := mediaItemMutationKeyFor(sectionTitle, existingItem)
			generation := c.itemMutationGeneration[oldKey]
			preserveDBOwnedMediaItemState(item, existingItem)
			preserveMediaItemVersion(item, existingItem)
			*existingItem = cloneMediaItem(item)
			if generation != 0 {
				delete(c.itemMutationGeneration, oldKey)
				c.itemMutationGeneration[mediaItemMutationKeyFor(sectionTitle, existingItem)] = generation
			}
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

func (c *MediaServerLibraryCache) nextDBMutationGenerationLocked() uint64 {
	c.dbMutationGeneration++
	return c.dbMutationGeneration
}

func (c *MediaServerLibraryCache) SetIgnored(sectionTitle, tmdbID string, ignored bool, mode string, ignoredSets ...[]string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return false
	}
	matched := false
	for i := range section.MediaItems {
		if section.MediaItems[i].TMDB_ID == tmdbID {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	generation := c.nextDBMutationGenerationLocked()
	var sets []string
	if ignored && len(ignoredSets) > 0 {
		sets = ignoredSets[0]
	}
	for i := range section.MediaItems {
		item := &section.MediaItems[i]
		if item.TMDB_ID == tmdbID {
			item.IgnoredInDB = ignored
			item.IgnoredMode = mode
			item.IgnoredSets = append([]string(nil), sets...)
			c.itemMutationGeneration[mediaItemMutationKeyFor(sectionTitle, item)] = generation
		}
	}
	return true
}

func (c *MediaServerLibraryCache) UpdateMediaItemDBSavedSets(sectionTitle string, item *models.MediaItem, dbSavedSets []models.DBSavedSet) {
	c.updateMediaItemDBSavedSets(sectionTitle, item, dbSavedSets, false)
}

func (c *MediaServerLibraryCache) UpsertMediaItemDBSavedSets(sectionTitle string, item *models.MediaItem, dbSavedSets []models.DBSavedSet) {
	c.updateMediaItemDBSavedSets(sectionTitle, item, dbSavedSets, true)
}

func (c *MediaServerLibraryCache) updateMediaItemDBSavedSets(sectionTitle string, item *models.MediaItem, dbSavedSets []models.DBSavedSet, insertMissing bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	section, exists := c.sections[sectionTitle]
	if !exists {
		return
	}
	matched := false
	for i := range section.MediaItems {
		if section.MediaItems[i].TMDB_ID == item.TMDB_ID {
			matched = true
			break
		}
	}
	if !matched && !insertMissing {
		return
	}
	generation := c.nextDBMutationGenerationLocked()
	for i := range section.MediaItems {
		existingItem := &section.MediaItems[i]
		if existingItem.TMDB_ID == item.TMDB_ID {
			existingItem.DBSavedSets = append([]models.DBSavedSet(nil), dbSavedSets...)
			c.itemMutationGeneration[mediaItemMutationKeyFor(sectionTitle, existingItem)] = generation
		}
	}
	if !matched {
		inserted := cloneMediaItem(item)
		inserted.DBSavedSets = append([]models.DBSavedSet(nil), dbSavedSets...)
		section.MediaItems = append(section.MediaItems, inserted)
		section.TotalSize = len(section.MediaItems)
		c.itemMutationGeneration[mediaItemMutationKeyFor(sectionTitle, &inserted)] = generation
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
