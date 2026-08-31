"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetLibrarySectionItems } from "@/services/mediaserver/get-library-section-items";
import { GetLibrarySections } from "@/services/mediaserver/get-library-sections";

import { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterHome } from "@/components/shared/filter-home";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { HomeMediaItemCardSkeletonGrid } from "@/components/shared/media-item-card-skeleton";
import { RefreshButton } from "@/components/shared/refresh-button";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";

import { log } from "@/lib/logger";
import { MAX_CACHE_DURATION, useLibrarySectionsStore } from "@/lib/stores/global-store-library-sections";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useHomePageStore } from "@/lib/stores/page-store-home";

import { extractInfoFromSearchQuery } from "@/hooks/search-query";

import type { APIResponse } from "@/types/api/api-response";
import type { LibrarySection } from "@/types/media-and-posters/media-item-and-library";

export default function Home() {
  const requestID = useRef(0);
  const { searchQuery } = useSearchQueryStore();
  const prevSearchQuery = useRef(searchQuery);

  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [sectionsLoaded, setSectionsLoaded] = useState(false);
  const [librarySections, setLibrarySections] = useState<LibrarySection[]>([]);
  const [totalItems, setTotalItems] = useState(0);
  const [hasUpdatedAt, setHasUpdatedAt] = useState(false);
  const [hasEpisodeAddedAt, setHasEpisodeAddedAt] = useState(false);

  const {
    filteredLibraries,
    setFilteredLibraries,
    filterInDB,
    setFilterInDB,
    filterIgnored,
    setFilterIgnored,
    hasSetsAvailableFilter,
    setHasSetsAvailableFilter,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
    filteredAndSortedMediaItems,
    setFilteredAndSortedMediaItems,
  } = useHomePageStore();

  const { sections, setSections, timestamp } = useLibrarySectionsStore();
  const hasHydrated = useLibrarySectionsStore((state) => state.hasHydrated);
  const totalPages = Math.ceil(totalItems / itemsPerPage);

  useEffect(() => {
    if (
      sortOption !== "title" &&
      sortOption !== "dateUpdated" &&
      sortOption !== "dateAdded" &&
      sortOption !== "dateReleased" &&
      sortOption !== "newEpisodeAdded"
    ) {
      setSortOption("dateAdded");
      setSortOrder("desc");
    }
  }, [sortOption, setSortOption, setSortOrder]);

  useEffect(() => {
    if (!hasHydrated || sectionsLoaded) return;

    const loadSections = async () => {
      const cachedSections = Object.values(sections);
      const cacheValid = Boolean(timestamp && Date.now() - timestamp < MAX_CACHE_DURATION && cachedSections.length > 0);
      if (cacheValid) {
        const summaries = cachedSections.map((section) => ({ ...section, media_items: [] }));
        setLibrarySections(summaries.sort((a, b) => a.title.localeCompare(b.title)));
        setSectionsLoaded(true);
        return;
      }

      const response = await GetLibrarySections();
      if (response.status === "error") {
        setError(response);
        setLoading(false);
        return;
      }
      const fetchedSections = (response.data?.sections ?? [])
        .map((section) => ({ ...section, media_items: [] }))
        .sort((a, b) => a.title.localeCompare(b.title));
      if (fetchedSections.length === 0) {
        setError(ReturnErrorMessage<unknown>(new Error("No sections found, please check the logs.")));
        setLoading(false);
        return;
      }
      setLibrarySections(fetchedSections);
      setSections(
        Object.fromEntries(
          fetchedSections.map((section) => [
            section.title,
            { ...section, media_items: sections[section.title]?.media_items ?? [] },
          ])
        ),
        Date.now()
      );
      setSectionsLoaded(true);
    };

    void loadSections();
  }, [hasHydrated, sections, sectionsLoaded, setSections, timestamp]);

  const fetchPage = useCallback(
    async (refresh = false) => {
      if (!sectionsLoaded) return;
      const thisRequest = ++requestID.current;
      setLoading(true);
      setError(null);
      const { searchTMDBID, searchLibrary, searchYear, searchTitle } = extractInfoFromSearchQuery(searchQuery);
      const response = await GetLibrarySectionItems({
        libraryTitles: filteredLibraries,
        searchTitle,
        searchLibrary,
        searchID: searchTMDBID,
        searchYear,
        filterInDB,
        filterIgnored,
        filterHasSets: hasSetsAvailableFilter,
        sortOption,
        sortOrder,
        pageNumber: currentPage,
        itemsPerPage,
        refresh,
      });
      if (thisRequest !== requestID.current) return;
      if (response.status === "error") {
        setError(response);
        setLoading(false);
        return;
      }

      const data = response.data;
      const nextTotal = data?.total_items ?? 0;
      const nextTotalPages = Math.ceil(nextTotal / itemsPerPage);
      if (nextTotalPages > 0 && currentPage > nextTotalPages) {
        setCurrentPage(nextTotalPages);
        return;
      }
      setFilteredAndSortedMediaItems(data?.media_items ?? []);
      setTotalItems(nextTotal);
      setHasUpdatedAt(data?.has_updated_at ?? false);
      setHasEpisodeAddedAt(data?.has_episode_added_at ?? false);
      setLoading(false);
      log("INFO", "Home Page", "Library Page", `Loaded ${data?.media_items.length ?? 0} of ${nextTotal} items`);
    },
    [
      currentPage,
      filterIgnored,
      filterInDB,
      filteredLibraries,
      hasSetsAvailableFilter,
      itemsPerPage,
      searchQuery,
      sectionsLoaded,
      setCurrentPage,
      setFilteredAndSortedMediaItems,
      sortOption,
      sortOrder,
    ]
  );

  useEffect(() => {
    void fetchPage();
  }, [fetchPage]);

  useEffect(() => {
    if (searchQuery !== prevSearchQuery.current) {
      setCurrentPage(1);
      prevSearchQuery.current = searchQuery;
    }
  }, [searchQuery, setCurrentPage]);

  if (error) {
    return <ErrorMessage error={error} />;
  }

  return (
    <div className="flex items-center justify-center">
      <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
        <div className="w-full flex items-center justify-center mb-4 mt-4">
          <FilterHome
            librarySections={librarySections}
            filteredLibraries={filteredLibraries}
            setFilteredLibraries={setFilteredLibraries}
            filterInDB={filterInDB}
            setFilterInDB={setFilterInDB}
            filterIgnored={filterIgnored}
            setFilterIgnored={setFilterIgnored}
            hasSetsAvailableFilter={hasSetsAvailableFilter}
            setHasSetsAvailableFilter={setHasSetsAvailableFilter}
            hasUpdatedAt={hasUpdatedAt}
            hasEpisodeAddedAt={hasEpisodeAddedAt}
            sortOption={sortOption}
            setSortOption={setSortOption}
            sortOrder={sortOrder}
            setSortOrder={setSortOrder}
            setCurrentPage={setCurrentPage}
            itemsPerPage={itemsPerPage}
            setItemsPerPage={setItemsPerPage}
          />
        </div>

        {loading ? (
          <HomeMediaItemCardSkeletonGrid />
        ) : (
          <ResponsiveGrid size="regular">
            {filteredAndSortedMediaItems.length === 0 && (searchQuery || filteredLibraries.length > 0) ? (
              <div className="col-span-full text-center text-red-500">
                <ErrorMessage
                  error={ReturnErrorMessage<string>(
                    `No items found${searchQuery ? ` matching "${searchQuery}"` : ""} in ${
                      filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"
                    }${
                      filterInDB === "notInDB"
                        ? " that are not in the database."
                        : filterInDB === "inDB"
                          ? " that are already in the database."
                          : ""
                    }`
                  )}
                />
              </div>
            ) : (
              filteredAndSortedMediaItems.map((item) => <HomeMediaItemCard key={item.rating_key} item={item} />)
            )}
          </ResponsiveGrid>
        )}

        <CustomPagination
          currentPage={currentPage}
          totalPages={totalPages}
          setCurrentPage={setCurrentPage}
          scrollToTop={true}
          filterItemsLength={totalItems}
          itemsPerPage={itemsPerPage}
        />
        <RefreshButton onClick={() => void fetchPage(true)} />
      </div>
    </div>
  );
}
