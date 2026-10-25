import { create } from "zustand";
import { persist } from "zustand/middleware";

import { PageStore } from "@/lib/stores/stores";

import type { SortStore } from "@/types/store-interfaces";
import type { TYPE_SORT_ORDER_OPTIONS } from "@/types/ui-options";

interface CollectionItem_PageStore extends SortStore<string, TYPE_SORT_ORDER_OPTIONS> {
  showHiddenUsers: boolean;
  setShowHiddenUsers: (show: boolean) => void;

  // Hydrate and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useCollectionItemPageStore = create<CollectionItem_PageStore>()(
  persist(
    (set) => ({
      sortOption: "dateUpdated",
      setSortOption: (option) => set({ sortOption: option }),

      sortOrder: "desc",
      setSortOrder: (order) => set({ sortOrder: order }),

      showHiddenUsers: false,
      setShowHiddenUsers: (show) => set({ showHiddenUsers: show }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      clear: () =>
        set({
          sortOption: "dateUpdated",
          sortOrder: "desc",
          showHiddenUsers: false,
          hasHydrated: false,
        }),
    }),
    {
      name: "CollectionItem",
      storage: PageStore,
      partialize: (state) => ({
        sortOption: state.sortOption,
        sortOrder: state.sortOrder,
        showHiddenUsers: state.showHiddenUsers,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
