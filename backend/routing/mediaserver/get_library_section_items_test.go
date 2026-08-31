package routes_ms

import (
	"aura/cache"
	"aura/models"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilterSortAndPaginateLibraryItems(t *testing.T) {
	items := []models.MediaItem{
		{TMDB_ID: "1", LibraryTitle: "Shows", Type: "show", Title: "Alpha Show", Year: 2020, AddedAt: 20, LatestEpisodeAddedAt: 25, HasMediuxSets: true},
		{TMDB_ID: "2", LibraryTitle: "Movies", Type: "movie", Title: "Beta Movie", Year: 2021, AddedAt: 30, DBSavedSets: []models.DBSavedSet{{ID: "set-1"}}},
		{TMDB_ID: "3", LibraryTitle: "Movies", Type: "movie", Title: "Gamma Movie", Year: 2021, AddedAt: 10, IgnoredInDB: true, IgnoredMode: "always"},
	}

	page := filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		LibraryTitles: []string{"Movies"},
		SearchTitle:   "movie",
		SearchYear:    2021,
		FilterInDB:    "notInDB",
		FilterIgnored: "not_ignored",
		SortOption:    "dateAdded",
		SortOrder:     "desc",
		PageNumber:    1,
		ItemsPerPage:  1,
	})

	if page.TotalItems != 0 {
		t.Fatalf("TotalItems = %d, want 0 after combined filters", page.TotalItems)
	}

	page = filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		LibraryTitles: []string{"Movies"},
		SearchTitle:   "movie",
		SearchYear:    2021,
		SortOption:    "dateAdded",
		SortOrder:     "desc",
		PageNumber:    1,
		ItemsPerPage:  1,
	})
	if page.TotalItems != 2 || len(page.Items) != 1 || page.Items[0].TMDB_ID != "2" {
		t.Fatalf("page = %+v, want first of 2 filtered items to be TMDB 2", page)
	}
	if page.HasUpdatedAt || page.HasEpisodeAddedAt {
		t.Fatalf("capability flags = (%v, %v), want no dates in filtered movie results", page.HasUpdatedAt, page.HasEpisodeAddedAt)
	}

	page = filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		LibraryTitles: []string{"Shows"}, PageNumber: 1, ItemsPerPage: 20,
	})
	if page.HasUpdatedAt || !page.HasEpisodeAddedAt {
		t.Fatalf("show capability flags = (%v, %v), want episode dates only", page.HasUpdatedAt, page.HasEpisodeAddedAt)
	}
}

func TestFilterSortAndPaginateLibraryItemsPreservesTitleOrderAndPageBounds(t *testing.T) {
	items := []models.MediaItem{
		{TMDB_ID: "1", Title: "beta"},
		{TMDB_ID: "2", Title: "Alpha"},
		{TMDB_ID: "3", Title: "charlie"},
	}

	page := filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		SortOption: "title", SortOrder: "asc", PageNumber: 2, ItemsPerPage: 1,
	})
	if page.TotalItems != 3 || len(page.Items) != 1 || page.Items[0].Title != "beta" {
		t.Fatalf("second title page = %+v, want beta from 3 case-insensitively sorted items", page)
	}

	page = filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		SortOption: "title", SortOrder: "asc", PageNumber: 4, ItemsPerPage: 1,
	})
	if page.TotalItems != 3 || len(page.Items) != 0 {
		t.Fatalf("page beyond filtered items = %+v, want empty page with total 3", page)
	}
}

func TestGetLibrarySectionItemsRefreshesColdCacheAndReportsFailure(t *testing.T) {
	previousCache := cache.LibraryStore
	previousRefresh := refreshLibraryCache
	t.Cleanup(func() {
		cache.LibraryStore = previousCache
		refreshLibraryCache = previousRefresh
	})
	cache.LibraryStore = cache.Cache_NewLibraryCache()

	refreshCalls := 0
	refreshLibraryCache = func(_ context.Context, force bool) bool {
		refreshCalls++
		if force {
			t.Fatal("cold-cache retry must not force a second full refresh")
		}
		return false
	}

	response := httptest.NewRecorder()
	GetLibrarySectionItems(response, httptest.NewRequest("GET", "/api/mediaserver/library/items", nil))
	if refreshCalls != 1 {
		t.Fatalf("cold-cache refresh calls = %d, want 1", refreshCalls)
	}
	if !strings.Contains(response.Body.String(), "Failed to refresh library items") {
		t.Fatalf("cold-cache response = %s, want explicit refresh error", response.Body.String())
	}
}

func TestFilterSortAndPaginateLibraryItemsPreservesSearchSemantics(t *testing.T) {
	items := []models.MediaItem{
		{TMDB_ID: "10", LibraryTitle: "4K Movies", Title: "Spider-Man: Homecoming", Year: 2017},
		{TMDB_ID: "11", LibraryTitle: "Movies", Title: "Spider Man No Way Home", Year: 2021},
	}

	page := filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		SearchTitle:   "spider home",
		SearchLibrary: "4k",
		SearchID:      "10",
		PageNumber:    1,
		ItemsPerPage:  20,
	})
	if page.TotalItems != 1 || page.Items[0].TMDB_ID != "10" {
		t.Fatalf("search page = %+v, want TMDB 10", page)
	}

	page = filterSortAndPaginateLibraryItems(items, libraryItemsQuery{
		SearchTitle:  `“Spider-Man: Homecoming”`,
		PageNumber:   1,
		ItemsPerPage: 20,
	})
	if page.TotalItems != 1 || page.Items[0].TMDB_ID != "10" {
		t.Fatalf("exact normalized-title search page = %+v, want TMDB 10", page)
	}
}
