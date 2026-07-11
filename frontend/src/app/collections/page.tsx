"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetMovieCollections } from "@/services/mediaserver/get-movie-collections";

import { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterCollections } from "@/components/shared/filter-collections";
import Loader from "@/components/shared/loader";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { RefreshButton } from "@/components/shared/refresh-button";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";

import { log } from "@/lib/logger";
import { useCollectionStore } from "@/lib/stores/global-store-collection-store";
import { MAX_CACHE_DURATION } from "@/lib/stores/global-store-library-sections";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useCollectionsPageStore } from "@/lib/stores/page-store-collections";

import { searchItems } from "@/hooks/search-query";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionItem } from "@/types/media-and-posters/collection-item";

// Backward-compatible re-export: CollectionItem now lives in
// @/types/media-and-posters/collection-item. Prefer importing it from there.
// This re-export keeps any remaining "@/app/collections/page" imports working
// during migration and can be removed once every consumer has moved over.
export type { CollectionItem };

export default function CollectionsPage() {
  const isMounted = useRef(false);

  // -------------------------------
  // States
  // -------------------------------
  // Search
  const { searchQuery } = useSearchQueryStore();
  const prevSearchQuery = useRef(searchQuery);

  // Loading & Error
  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const [fullyLoaded, setFullyLoaded] = useState<boolean>(false);

  const [collectionItems, setCollectionItems] = useState<CollectionItem[]>([]);

  const {
    collectionItems: storedCollectionItems,
    setCollectionItems: setStoredCollectionItems,
    timestamp: storedTimestamp,
  } = useCollectionStore();

  // State to track the CollectionsPageStore values
  const {
    filteredLibraries,
    setFilteredLibraries,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
    filteredAndSortedCollectionItems,
    setFilteredAndSortedCollectionItems,
  } = useCollectionsPageStore();

  // -------------------------------
  // Derived values
  // -------------------------------
  const paginatedItems = filteredAndSortedCollectionItems.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );
  const totalPages = Math.ceil(filteredAndSortedCollectionItems.length / itemsPerPage);

  // Set sortOption to "title" if its not title, numberOfItems
  useEffect(() => {
    if (sortOption !== "title" && sortOption !== "numberOfItems") {
      setSortOption("title");
      setSortOrder("desc");
    }
  }, [sortOption, setSortOption, setSortOrder]);

  // Fetch data from cache or API
  const fetchCollections = useCallback(
    async (useCache: boolean) => {
      if (isMounted.current && useCache) return;
      setError(null);
      setFullyLoaded(false);
      setCollectionItems([]);
      try {
        // Check if we want to use cache
        if (useCache) {
          const isCacheAgeValid = storedTimestamp ? Date.now() - storedTimestamp < MAX_CACHE_DURATION : false;
          const cacheContainsCollectionItemsAndTimestamp =
            storedCollectionItems && storedTimestamp && storedCollectionItems.length > 0;
          log("INFO", "Collections Page", "Library Cache", "Attempting to load collection items from cache", {
            "Current Time": Date.now(),
            "Cache Timestamp": storedTimestamp,
            "Cache Age Max (ms)": MAX_CACHE_DURATION,
            "Cache Age (ms)": storedTimestamp ? Date.now() - storedTimestamp : "N/A",
            "Is Cache Age Valid": isCacheAgeValid,
            "Cache Contains Collection Items & Timestamp": cacheContainsCollectionItemsAndTimestamp,
          });
          if (cacheContainsCollectionItemsAndTimestamp) {
            if (isCacheAgeValid) {
              setCollectionItems(storedCollectionItems);
              setFullyLoaded(true);
              log("INFO", "Collections Page", "Library Cache", "Loaded collection items from cache", {
                "Number of Items": storedCollectionItems.length,
              });
              return;
            } else {
              log(
                "WARN",
                "Collections Page",
                "Library Cache",
                "Cache is stale, fetching fresh collection items from API"
              );
            }
          } else {
            log(
              "WARN",
              "Collections Page",
              "Library Cache",
              "No valid cache found, fetching collection items from API"
            );
          }
        }

        const response = await GetMovieCollections();
        if (response.status === "error") {
          setError(response);
          setFullyLoaded(true);
          return;
        }

        const fetchedCollectionItems = response.data?.collections || [];
        if (!fetchedCollectionItems || fetchedCollectionItems.length === 0) {
          setError(ReturnErrorMessage("No Collection Items found in Media Server"));
          return;
        }

        log("INFO", "Collections Page", "Fetched Collection Items:", `Fetched ${fetchedCollectionItems.length} items`, {
          fetchedCollectionItems,
        });

        // Store in global store for caching
        setStoredCollectionItems(fetchedCollectionItems, Date.now());

        setCollectionItems(fetchedCollectionItems);
        setFullyLoaded(true);
      } catch (error) {
        setError(ReturnErrorMessage<unknown>(error));
      } finally {
        isMounted.current = false;
      }
    },
    [setStoredCollectionItems, storedCollectionItems, storedTimestamp]
  );

  useEffect(() => {
    fetchCollections(true);
    isMounted.current = true;
  }, [fetchCollections]);

  useEffect(() => {
    if (searchQuery !== prevSearchQuery.current) {
      setCurrentPage(1);
      prevSearchQuery.current = searchQuery;
    }
  }, [searchQuery, setCurrentPage]);

  // Filter Items
  useEffect(() => {
    const filterAndSortItems = async () => {
      let items = [...collectionItems];

      // Filter out items with no ChildCount
      items = items.filter((item) => item.child_count > 0);

      // Sort items by Title
      if (sortOption === "title") {
        if (sortOrder === "asc") {
          items.sort((a, b) => a.title.localeCompare(b.title));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => b.title.localeCompare(a.title));
        }
      } else if (sortOption === "numberOfItems") {
        if (sortOrder === "asc") {
          items.sort((a, b) => a.child_count - b.child_count);
        } else if (sortOrder === "desc") {
          items.sort((a, b) => b.child_count - a.child_count);
        }
      }

      // Filter by Libraries
      if (filteredLibraries.length > 0) {
        items = items.filter((item) => item.library_title && filteredLibraries.includes(item.library_title));
      }

      // Filter out items by search
      const filteredItems = searchItems(items, searchQuery, {
        getTitle: (item) => item.title,
        getLibraryTitle: (item) => item.library_title,
        getID: (item) => item.rating_key,
      });

      // Store the filtered and sorted items in local storage
      setFilteredAndSortedCollectionItems(filteredItems);
    };

    filterAndSortItems();
  }, [collectionItems, filteredLibraries, sortOption, sortOrder, setFilteredAndSortedCollectionItems, searchQuery]);

  if (error) {
    return <ErrorMessage error={error} />;
  }

  return (
    <div className="flex items-center justify-center">
      {fullyLoaded ? (
        <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
          {/* Filter & Sort Controls */}
          <div className="w-full flex items-center justify-center mb-4 mt-4">
            <FilterCollections
              librarySections={[...new Set(collectionItems.map((item) => item.library_title || "Unknown"))]}
              filteredLibraries={filteredLibraries}
              setFilteredLibraries={setFilteredLibraries}
              sortOption={sortOption}
              setSortOption={setSortOption}
              sortOrder={sortOrder}
              setSortOrder={setSortOrder}
              setCurrentPage={setCurrentPage}
              itemsPerPage={itemsPerPage}
              setItemsPerPage={setItemsPerPage}
            />
          </div>

          {/* Grid of Cards */}
          <ResponsiveGrid size="regular">
            {paginatedItems.length === 0 && fullyLoaded && (searchQuery || filteredLibraries.length > 0) ? (
              <div className="col-span-full text-center text-red-500">
                <ErrorMessage
                  error={ReturnErrorMessage<string>(
                    `No collection items found${searchQuery ? ` matching "${searchQuery}"` : ""} in ${
                      filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"
                    }`
                  )}
                />
              </div>
            ) : (
              paginatedItems.map((item) => <HomeMediaItemCard key={item.rating_key} item={item} />)
            )}
          </ResponsiveGrid>

          {/* Pagination */}
          <CustomPagination
            currentPage={currentPage}
            totalPages={totalPages}
            setCurrentPage={setCurrentPage}
            scrollToTop={true}
            filterItemsLength={filteredAndSortedCollectionItems.length}
            itemsPerPage={itemsPerPage}
          />

          {/* Refresh Button */}
          <RefreshButton onClick={() => fetchCollections(false)} />
        </div>
      ) : (
        <Loader className="mt-20" message="Loading Collection Items" />
      )}
    </div>
  );
}
