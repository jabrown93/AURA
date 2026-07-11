import { create } from "zustand";
import { persist } from "zustand/middleware";

import { PageStore } from "@/lib/stores/stores";

import type { CollectionItem } from "@/types/media-and-posters/collection-item";
import type { PaginationStore, SortStore } from "@/types/store-interfaces";
import type { TYPE_SORT_ORDER_OPTIONS } from "@/types/ui-options";

type Direction = "next" | "previous";

interface CollectionsHome_PageStore
  extends SortStore<string, TYPE_SORT_ORDER_OPTIONS>, PaginationStore<number, number> {
  // Filters
  filteredAndSortedCollectionItems: CollectionItem[];
  setFilteredAndSortedCollectionItems: (items: CollectionItem[]) => void;
  filteredLibraries: string[];
  setFilteredLibraries: (libraries: string[]) => void;

  getAdjacentCollectionItem: (currentRatingKey: string, direction: Direction) => CollectionItem | null;

  // Adjacent Items
  previousCollectionItem: CollectionItem | null;
  setPreviousCollectionItem: (collectionItem: CollectionItem | null) => void;
  nextCollectionItem: CollectionItem | null;
  setNextCollectionItem: (collectionItem: CollectionItem | null) => void;

  // Hydrate and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useCollectionsPageStore = create<CollectionsHome_PageStore>()(
  persist(
    (set, get) => ({
      sortOption: "title",
      setSortOption: (option) => set({ sortOption: option }),

      sortOrder: "asc",
      setSortOrder: (order) => set({ sortOrder: order }),

      currentPage: 1,
      setCurrentPage: (page) => set({ currentPage: page }),

      itemsPerPage: 20,
      setItemsPerPage: (itemsPerPage) => set({ itemsPerPage }),

      filteredAndSortedCollectionItems: [],
      setFilteredAndSortedCollectionItems: (items) => set({ filteredAndSortedCollectionItems: items }),

      filteredLibraries: [],
      setFilteredLibraries: (libraries) => set({ filteredLibraries: libraries }),

      /**
       * Retrieves adjacent collection item (wrap-around) from the Collections page store's
       * filteredAndSortedCollectionItems array.
       */
      getAdjacentCollectionItem: (ratingKey: string, direction: Direction): CollectionItem | null => {
        const collectionItems = get().filteredAndSortedCollectionItems || [];
        if (!collectionItems.length) return null;

        const currentIndex = collectionItems.findIndex((m) => m.rating_key === ratingKey);
        if (currentIndex === -1) return null;

        const nextIndex =
          direction === "next"
            ? (currentIndex + 1) % collectionItems.length
            : (currentIndex - 1 + collectionItems.length) % collectionItems.length;

        return collectionItems[nextIndex] ?? null;
      },

      previousCollectionItem: null,
      setPreviousCollectionItem: (collectionItem) => set({ previousCollectionItem: collectionItem }),

      nextCollectionItem: null,
      setNextCollectionItem: (collectionItem) => set({ nextCollectionItem: collectionItem }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
      clear: () =>
        set({
          sortOption: "title",
          sortOrder: "asc",
          currentPage: 1,
          itemsPerPage: 20,
          filteredAndSortedCollectionItems: [],
          filteredLibraries: [],
          previousCollectionItem: null,
          nextCollectionItem: null,
          hasHydrated: false,
        }),
    }),
    {
      name: "Collections",
      storage: PageStore,
      partialize: (state) => ({
        sortOption: state.sortOption,
        sortOrder: state.sortOrder,
        currentPage: state.currentPage,
        itemsPerPage: state.itemsPerPage,
        filteredAndSortedCollectionItems: state.filteredAndSortedCollectionItems,
        filteredLibraries: state.filteredLibraries,
        previousCollectionItem: state.previousCollectionItem,
        nextCollectionItem: state.nextCollectionItem,
        hasHydrated: state.hasHydrated,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
