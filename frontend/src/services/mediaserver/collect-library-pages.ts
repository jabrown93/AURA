export interface LibraryItemsQuery {
  libraryTitles: string[];
  searchTitle: string;
  searchLibrary: string;
  searchID: string;
  searchYear: number;
  filterInDB: string;
  filterIgnored: string;
  filterHasSets: string;
  sortOption: string;
  sortOrder: "asc" | "desc";
  pageNumber: number;
  itemsPerPage: number;
  refresh?: boolean;
}

export function buildLibraryItemsParams(query: LibraryItemsQuery) {
  return {
    library_titles: query.libraryTitles,
    search_title: query.searchTitle,
    search_library: query.searchLibrary,
    search_id: query.searchID,
    search_year: query.searchYear,
    filter_in_db: query.filterInDB,
    filter_ignored: query.filterIgnored,
    filter_has_sets: query.filterHasSets,
    sort_option: query.sortOption,
    sort_order: query.sortOrder,
    page_number: query.pageNumber,
    items_per_page: query.itemsPerPage,
    refresh: query.refresh,
  };
}

export function beginLibraryMembershipFetch(
  attemptedLibraryTitles: Set<string>,
  libraryTitle: string,
  itemCount: number
): boolean {
  if (itemCount > 0 || attemptedLibraryTitles.has(libraryTitle)) return false;
  attemptedLibraryTitles.add(libraryTitle);
  return true;
}

export async function collectLibraryPages<T>(
  fetchPage: (pageNumber: number) => Promise<{ items: T[]; totalItems: number }>
): Promise<T[]> {
  const items: T[] = [];
  for (let pageNumber = 1; ; pageNumber += 1) {
    const page = await fetchPage(pageNumber);
    items.push(...page.items);
    if (page.items.length === 0 || items.length >= page.totalItems) return items;
  }
}
