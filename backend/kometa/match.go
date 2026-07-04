package kometa

import (
	"aura/cache"
	"aura/models"
)

// matchFolderToItems resolves a Kometa asset folder name to media items in the library
// cache. It tries, in order: a {tmdb-NNN} hint, then a "Title (Year)" parse. Because a
// shared asset directory can serve multiple libraries (e.g. HD and 4K), all matching items
// across sections are returned.
func matchFolderToItems(folderName string) []*models.MediaItem {
	sections := cache.LibraryStore.GetAllSectionsSortedByTitle()

	if tmdbID, ok := parseTMDBHint(folderName); ok {
		var matches []*models.MediaItem
		for _, section := range sections {
			if item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBID(section.Title, tmdbID); found {
				matches = append(matches, item)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}

	if title, year, ok := parseTitleYear(folderName); ok {
		var matches []*models.MediaItem
		for _, section := range sections {
			if item, found := cache.LibraryStore.GetMediaItemFromSectionByTitleAndYear(section.Title, title, year); found {
				matches = append(matches, item)
			}
		}
		return matches
	}

	return nil
}

// matchFolderToCollection resolves a Kometa asset folder name to a media-server collection
// by normalized title comparison.
func matchFolderToCollection(folderName string) (*models.CollectionItem, bool) {
	target := normalizeTitle(folderName)
	if target == "" {
		return nil, false
	}
	for _, collection := range cache.CollectionsStore.GetAllCollections() {
		if normalizeTitle(collection.Title) == target {
			c := collection
			return &c, true
		}
	}
	return nil, false
}
