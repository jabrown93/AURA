package creator

import (
	"aura/cache"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// A creator's sets are kept for a while, so MediUX isn't asked on every page
const setsMaxAge = 10 * time.Minute

type savedSets struct {
	sets      models.CreatorSetsResponse
	fetchedAt time.Time
}

var (
	setsMu    sync.Mutex
	setsCache = map[string]savedSets{}
)

// Replaced in tests
var getAllUserSets = mediux.GetAllUserSets

// creatorSets returns the creator's sets, from MediUX if they aren't kept or are too old
func creatorSets(ctx context.Context, username string, refresh bool) (models.CreatorSetsResponse, logging.LogErrorInfo) {
	key := strings.ToLower(username)
	setsMu.Lock()
	saved, ok := setsCache[key]
	setsMu.Unlock()
	if ok && !refresh && time.Since(saved.fetchedAt) < setsMaxAge {
		return saved.sets, logging.LogErrorInfo{}
	}

	sets, Err := getAllUserSets(ctx, username)
	if Err.Message != "" {
		return sets, Err
	}
	setsMu.Lock()
	setsCache[key] = savedSets{sets: sets, fetchedAt: time.Now()}
	setsMu.Unlock()
	return sets, logging.LogErrorInfo{}
}

// Tabs of the user page
const (
	TabShowSets       = "showSets"
	TabMovieSets      = "movieSets"
	TabCollectionSets = "collectionSets"
	TabBoxSets        = "boxSets"
)

// Query is what the user page asks for
type Query struct {
	Username string
	Library  string // The library to match the sets against, empty lists the libraries
	Tab      string // showSets, movieSets, collectionSets or boxSets
	Search   string // Set title or the title of an item in the set
	InDB     string // inDB, notInDB, otherSetInDB or empty
	Sort     string // title or dateLastUpdate
	Order    string // asc or desc
	Page     int    // From 1
	PerPage  int
	Refresh  bool // Get the sets from MediUX again
}

// LibrarySummary is a library and how many of the creator's sets match it
type LibrarySummary struct {
	Title string `json:"title"`
	Type  string `json:"type"`
	Sets  int    `json:"sets"`
}

// TabCounts is how many sets each tab has, after the filters
type TabCounts struct {
	ShowSets       int `json:"show_sets"`
	MovieSets      int `json:"movie_sets"`
	CollectionSets int `json:"collection_sets"`
	BoxSets        int `json:"box_sets"`
}

// Boxset is a boxset with its sets that match the library
type Boxset struct {
	models.BoxsetRef
	Sets []models.SetRef `json:"sets"`
}

// Result is one page of one tab, with the counts of every tab
type Result struct {
	Libraries     []LibrarySummary               `json:"libraries"`
	Totals        TabCounts                      `json:"totals"` // Sets of each type the creator has
	Counts        TabCounts                      `json:"counts"` // Sets of each type matching the library and filters
	Tab           string                         `json:"tab"`
	Sets          []models.SetRef                `json:"sets"`    // The page, for the set tabs
	Boxsets       []Boxset                       `json:"boxsets"` // The page, for the boxsets tab
	IncludedItems map[string]models.IncludedItem `json:"included_items"`
	Total         int                            `json:"total"`
	Page          int                            `json:"page"`
	PerPage       int                            `json:"per_page"`
	TotalPages    int                            `json:"total_pages"`
}

const defaultPerPage = 20

// Find matches a creator's sets against a library and returns one page of a tab
func Find(ctx context.Context, store *cache.MediaServerLibraryCache, q Query) (Result, logging.LogErrorInfo) {
	result := Result{Libraries: []LibrarySummary{}, Sets: []models.SetRef{}, Boxsets: []Boxset{}, IncludedItems: map[string]models.IncludedItem{}, Tab: q.Tab}
	sets, Err := creatorSets(ctx, q.Username, q.Refresh)
	if Err.Message != "" {
		return result, Err
	}
	result.Totals = TabCounts{ShowSets: len(sets.ShowSets), MovieSets: len(sets.MovieSets), CollectionSets: len(sets.CollectionSets), BoxSets: len(sets.Boxsets)}

	libraries := libraryItems(store)
	for _, library := range libraries {
		m := match(sets, library)
		result.Libraries = append(result.Libraries, LibrarySummary{
			Title: library.title, Type: library.kind,
			Sets: len(m.showSets) + len(m.movieSets) + len(m.collectionSets),
		})
	}

	selected := -1
	for i, library := range libraries {
		if library.title == q.Library {
			selected = i
		}
	}
	if selected == -1 {
		result.Page, result.PerPage, result.TotalPages = 1, max(q.PerPage, defaultPerPage), 1
		return result, logging.LogErrorInfo{}
	}
	library := libraries[selected]
	m := match(sets, library)

	f := filter{query: q, lib: library}
	showSets := f.sets(m.showSets)
	movieSets := f.sets(m.movieSets)
	collectionSets := f.sets(m.collectionSets)
	boxsets := f.boxsets(m.boxsets)
	result.Counts = TabCounts{ShowSets: len(showSets), MovieSets: len(movieSets), CollectionSets: len(collectionSets), BoxSets: len(boxsets)}

	switch q.Tab {
	case TabShowSets, TabMovieSets, TabCollectionSets:
		tabSets := map[string][]models.SetRef{TabShowSets: showSets, TabMovieSets: movieSets, TabCollectionSets: collectionSets}[q.Tab]
		sortByTitleOrDate(tabSets, q, func(s models.SetRef) (string, time.Time) { return s.Title, setDate(s) })
		start, end := page(&result, len(tabSets), q)
		result.Sets = tabSets[start:end]
		for _, set := range result.Sets {
			addIncludedItems(&result, set, sets.IncludedItems, library.itemsFor(set.Type))
		}
	default:
		result.Tab = TabBoxSets
		sortByTitleOrDate(boxsets, q, func(b Boxset) (string, time.Time) { return b.Title, boxsetDate(b) })
		start, end := page(&result, len(boxsets), q)
		result.Boxsets = boxsets[start:end]
		for _, boxset := range result.Boxsets {
			for _, set := range boxset.Sets {
				addIncludedItems(&result, set, sets.IncludedItems, library.itemsFor(set.Type))
			}
		}
	}
	return result, logging.LogErrorInfo{}
}

// library is a library's movies and shows by TMDB ID, kept apart as a movie and a show can share an ID
type library struct {
	title  string
	kind   string // movie, show or mixed
	movies map[string]models.MediaItem
	shows  map[string]models.MediaItem
}

// itemsFor returns the library's items a set of this type can have
func (lib library) itemsFor(setType string) map[string]models.MediaItem {
	if setType == "show" {
		return lib.shows
	}
	return lib.movies
}

// libraryItems returns the movie, show and mixed libraries, with the first item of each TMDB ID
func libraryItems(store *cache.MediaServerLibraryCache) []library {
	var libraries []library
	index := map[string]int{}
	store.ForEachMediaItem(func(section models.LibrarySectionBase, item *models.MediaItem) {
		if section.Type != "movie" && section.Type != "show" && section.Type != "mixed" {
			return
		}
		i, ok := index[section.Title]
		if !ok {
			i = len(libraries)
			index[section.Title] = i
			libraries = append(libraries, library{title: section.Title, kind: section.Type, movies: map[string]models.MediaItem{}, shows: map[string]models.MediaItem{}})
		}
		// A mixed library has both, its items say which they are
		itemType := section.Type
		if itemType == "mixed" {
			itemType = item.Type
		}
		items := libraries[i].movies
		switch itemType {
		case "show":
			items = libraries[i].shows
		case "movie":
		default:
			return
		}
		if _, exists := items[item.TMDB_ID]; item.TMDB_ID != "" && !exists {
			items[item.TMDB_ID] = *item
		}
	})
	return libraries
}

// matched is the creator's sets that have an item in a library
type matched struct {
	showSets, movieSets, collectionSets []models.SetRef
	boxsets                             []Boxset
}

// match finds the creator's sets with an item in the library
func match(sets models.CreatorSetsResponse, lib library) matched {
	inLibrary := func(set models.SetRef) bool {
		items := lib.itemsFor(set.Type)
		return slices.ContainsFunc(set.ItemIDs, func(id string) bool {
			_, ok := items[id]
			return ok
		})
	}
	// Sets on their own also need MediUX's info for the item
	withInfo := func(list []models.SetRef) []models.SetRef {
		out := []models.SetRef{}
		for _, set := range list {
			items := lib.itemsFor(set.Type)
			if slices.ContainsFunc(set.ItemIDs, func(id string) bool {
				_, inLib := items[id]
				_, hasInfo := sets.IncludedItems[id]
				return inLib && hasInfo
			}) {
				out = append(out, set)
			}
		}
		return out
	}

	// A mixed library matches both show and movie sets
	m := matched{}
	setsByID := map[string]models.SetRef{}
	boxsetTypes := []string{}
	if len(lib.shows) > 0 {
		m.showSets = withInfo(sets.ShowSets)
		boxsetTypes = append(boxsetTypes, "show")
		for _, set := range sets.ShowSets {
			if inLibrary(set) {
				setsByID[set.Type+"|"+set.ID] = set
			}
		}
	}
	if len(lib.movies) > 0 {
		m.movieSets = withInfo(sets.MovieSets)
		m.collectionSets = withInfo(sets.CollectionSets)
		boxsetTypes = append(boxsetTypes, "movie", "collection")
		for _, set := range append(slices.Clone(sets.MovieSets), sets.CollectionSets...) {
			if inLibrary(set) {
				setsByID[set.Type+"|"+set.ID] = set
			}
		}
	}

	// Boxsets keep only their sets that match the library
	m.boxsets = []Boxset{}
	for _, boxset := range sets.Boxsets {
		b := Boxset{BoxsetRef: boxset, Sets: []models.SetRef{}}
		for _, setType := range boxsetTypes {
			for _, id := range boxset.SetIDs[setType] {
				if set, ok := setsByID[setType+"|"+id]; ok {
					b.Sets = append(b.Sets, set)
				}
			}
		}
		if len(b.Sets) > 0 {
			m.boxsets = append(m.boxsets, b)
		}
	}
	return m
}

// filter applies the search and the saved filter
type filter struct {
	query Query
	lib   library
}

func (f filter) sets(list []models.SetRef) []models.SetRef {
	out := []models.SetRef{}
	for _, set := range list {
		if f.matchesSearch(set.Title, set) && f.setSaved(set) {
			out = append(out, set)
		}
	}
	return out
}

func (f filter) boxsets(list []Boxset) []Boxset {
	out := []Boxset{}
	for _, boxset := range list {
		if !f.boxsetMatchesSearch(boxset) {
			continue
		}
		switch f.query.InDB {
		case "inDB":
			// Every set of the boxset is saved
			if !all(boxset.Sets, f.savedWithThisSet) {
				continue
			}
		case "notInDB":
			if !slices.ContainsFunc(boxset.Sets, f.notSaved) {
				continue
			}
		case "otherSetInDB":
			if !slices.ContainsFunc(boxset.Sets, f.savedWithOtherSet) {
				continue
			}
		}
		out = append(out, boxset)
	}
	return out
}

func (f filter) setSaved(set models.SetRef) bool {
	switch f.query.InDB {
	case "inDB":
		return f.savedWithThisSet(set)
	case "notInDB":
		return f.notSaved(set)
	case "otherSetInDB":
		return f.savedWithOtherSet(set)
	}
	return true
}

// savedWithThisSet is whether an item of the set is saved with this set
func (f filter) savedWithThisSet(set models.SetRef) bool {
	items := f.lib.itemsFor(set.Type)
	return slices.ContainsFunc(set.ItemIDs, func(id string) bool {
		item, ok := items[id]
		return ok && slices.ContainsFunc(item.DBSavedSets, func(saved models.DBSavedSet) bool { return saved.ID == set.ID })
	})
}

// notSaved is whether an item of the set isn't saved, or isn't in the library
func (f filter) notSaved(set models.SetRef) bool {
	items := f.lib.itemsFor(set.Type)
	return slices.ContainsFunc(set.ItemIDs, func(id string) bool {
		item, ok := items[id]
		return !ok || len(item.DBSavedSets) == 0
	})
}

// savedWithOtherSet is whether an item of the set is saved, but only with other creators' sets
func (f filter) savedWithOtherSet(set models.SetRef) bool {
	items := f.lib.itemsFor(set.Type)
	return slices.ContainsFunc(set.ItemIDs, func(id string) bool {
		item, ok := items[id]
		return ok && len(item.DBSavedSets) > 0 && all(item.DBSavedSets, func(saved models.DBSavedSet) bool {
			return saved.ID != set.ID && saved.UserCreated != f.query.Username
		})
	})
}

func (f filter) matchesSearch(title string, set models.SetRef) bool {
	search := strings.ToLower(strings.TrimSpace(f.query.Search))
	if search == "" || strings.Contains(strings.ToLower(title), search) {
		return true
	}
	items := f.lib.itemsFor(set.Type)
	return slices.ContainsFunc(set.ItemIDs, func(id string) bool {
		item, ok := items[id]
		return ok && strings.Contains(strings.ToLower(item.Title), search)
	})
}

func (f filter) boxsetMatchesSearch(boxset Boxset) bool {
	search := strings.ToLower(strings.TrimSpace(f.query.Search))
	if search == "" || strings.Contains(strings.ToLower(boxset.Title), search) {
		return true
	}
	return slices.ContainsFunc(boxset.Sets, func(set models.SetRef) bool { return f.matchesSearch(set.Title, set) })
}

// sortByTitleOrDate sorts by title, or by when the set was last updated
func sortByTitleOrDate[T any](list []T, q Query, key func(T) (string, time.Time)) {
	var compare func(a, b T) int
	if q.Sort == "title" {
		collator := collate.New(language.English)
		compare = func(a, b T) int {
			titleA, _ := key(a)
			titleB, _ := key(b)
			return collator.CompareString(titleA, titleB)
		}
	} else {
		compare = func(a, b T) int {
			_, dateA := key(a)
			_, dateB := key(b)
			return dateA.Compare(dateB)
		}
	}
	if q.Order != "asc" {
		ascending := compare
		compare = func(a, b T) int { return ascending(b, a) }
	}
	slices.SortStableFunc(list, compare)
}

// setDate is when the set was last updated, or created, or its first image was changed
func setDate(set models.SetRef) time.Time {
	switch {
	case !set.DateUpdated.IsZero():
		return set.DateUpdated
	case !set.DateCreated.IsZero():
		return set.DateCreated
	case len(set.Images) > 0:
		return set.Images[0].Modified
	}
	return time.Time{}
}

func boxsetDate(boxset Boxset) time.Time {
	if !boxset.DateUpdated.IsZero() {
		return boxset.DateUpdated
	}
	return boxset.DateCreated
}

// page sets the page numbers and returns where the page starts and ends
func page(result *Result, total int, q Query) (start, end int) {
	result.PerPage = q.PerPage
	if result.PerPage <= 0 {
		result.PerPage = defaultPerPage
	}
	result.Total = total
	result.TotalPages = max(1, (total+result.PerPage-1)/result.PerPage)
	result.Page = min(max(q.Page, 1), result.TotalPages)
	start = min((result.Page-1)*result.PerPage, total)
	end = min(start+result.PerPage, total)
	return start, end
}

// addIncludedItems adds MediUX's info and the library item for each item of the set in the library
func addIncludedItems(result *Result, set models.SetRef, info map[string]models.IncludedItem, items map[string]models.MediaItem) {
	for _, id := range set.ItemIDs {
		item, inLibrary := items[id]
		included, hasInfo := info[id]
		if !inLibrary || !hasInfo {
			continue
		}
		included.MediaItem = item
		result.IncludedItems[id] = included
	}
}

func all[T any](list []T, fn func(T) bool) bool {
	for _, v := range list {
		if !fn(v) {
			return false
		}
	}
	return true
}

// Sets returns a creator's sets, kept for a few minutes
func Sets(ctx context.Context, username string) (models.CreatorSetsResponse, logging.LogErrorInfo) {
	return creatorSets(ctx, username, false)
}
