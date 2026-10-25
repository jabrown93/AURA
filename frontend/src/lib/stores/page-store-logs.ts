import { create } from "zustand";
import { persist } from "zustand/middleware";

import { syncPageView } from "@/lib/stores/page-view-sync";
import { PageStore } from "@/lib/stores/stores";

import type { PaginationStore } from "@/types/store-interfaces";

interface Logs_PageStore extends PaginationStore<number, number> {
  // Filters
  levelsFilter: string[];
  setLevelsFilter: (levels: string[]) => void;

  statusFilter: string[];
  setStatusFilter: (status: string[]) => void;

  actionsFilter: string[];
  setActionsFilter: (actions: string[]) => void;

  // Hydration and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useLogsPageStore = create<Logs_PageStore>()(
  persist(
    (set) => ({
      currentPage: 1,
      setCurrentPage: (page) => set({ currentPage: page }),

      itemsPerPage: 20,
      setItemsPerPage: (itemsPerPage) => set({ itemsPerPage }),

      levelsFilter: [],
      setLevelsFilter: (levels) => set({ levelsFilter: levels }),

      statusFilter: [],
      setStatusFilter: (status) => set({ statusFilter: status }),

      actionsFilter: [],
      setActionsFilter: (actions) => set({ actionsFilter: actions }),

      // Hydration and Clear
      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      clear: () =>
        set({
          levelsFilter: [],
          statusFilter: [],
          actionsFilter: [],
          hasHydrated: false,
        }),
    }),
    {
      name: "Logs",
      storage: PageStore,
      partialize: (state) => ({
        levelsFilter: state.levelsFilter,
        statusFilter: state.statusFilter,
        actionsFilter: state.actionsFilter,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);

// The sort and filter options are the same on every device
syncPageView("logs", useLogsPageStore, ["levelsFilter", "statusFilter", "actionsFilter"]);
