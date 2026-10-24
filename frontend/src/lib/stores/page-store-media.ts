import { create } from "zustand";
import { persist } from "zustand/middleware";

import { syncPageView } from "@/lib/stores/page-view-sync";
import { PageStore } from "@/lib/stores/stores";

import type { TYPE_SORT_ORDER_OPTIONS } from "@/types/ui-options";

type MediaType = "movie" | "show";

interface SortState {
  sortOption: string;
  sortOrder: TYPE_SORT_ORDER_OPTIONS;
}

interface Media_PageStore {
  sortStates: Record<MediaType, SortState>;
  setSortOption: (type: MediaType, option: string) => void;
  setSortOrder: (type: MediaType, order: TYPE_SORT_ORDER_OPTIONS) => void;

  // Filters
  showHiddenUsers: boolean;
  setShowHiddenUsers: (show: boolean) => void;
  showOnlyTitlecardSets: boolean;
  setShowOnlyTitlecardSets: (show: boolean) => void;

  filterByLanguage: string[];
  setFilterByLanguage: (languages: string[]) => void;

  // Hydrate and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useMediaPageStore = create<Media_PageStore>()(
  persist(
    (set) => ({
      sortStates: {
        movie: { sortOption: "date", sortOrder: "desc" },
        show: { sortOption: "date", sortOrder: "desc" },
      },
      setSortOption: (type, option) =>
        set((state) => ({
          sortStates: {
            ...state.sortStates,
            [type]: { ...state.sortStates[type], sortOption: option },
          },
        })),
      setSortOrder: (type, order) =>
        set((state) => ({
          sortStates: {
            ...state.sortStates,
            [type]: { ...state.sortStates[type], sortOrder: order },
          },
        })),

      showHiddenUsers: false,
      setShowHiddenUsers: (show) => set({ showHiddenUsers: show }),

      showOnlyTitlecardSets: false,
      setShowOnlyTitlecardSets: (show) => set({ showOnlyTitlecardSets: show }),

      filterByLanguage: [""],
      setFilterByLanguage: (languages) => set({ filterByLanguage: languages }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      clear: () =>
        set({
          sortStates: {
            movie: { sortOption: "date", sortOrder: "desc" },
            show: { sortOption: "date", sortOrder: "desc" },
          },
          showHiddenUsers: false,
          showOnlyTitlecardSets: false,
          filterByLanguage: [""],
        }),
    }),
    {
      name: "MediaItem",
      storage: PageStore,
      partialize: (state) => ({
        sortStates: state.sortStates,
        showHiddenUsers: state.showHiddenUsers,
        showOnlyTitlecardSets: state.showOnlyTitlecardSets,
        filterByLanguage: state.filterByLanguage,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);

// The sort and filter options are the same on every device
syncPageView("media_item", useMediaPageStore, [
  "sortStates",
  "showHiddenUsers",
  "showOnlyTitlecardSets",
  "filterByLanguage",
]);
