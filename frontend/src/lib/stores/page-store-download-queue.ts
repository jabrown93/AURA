import { create } from "zustand";
import { persist } from "zustand/middleware";

import { syncPageView } from "@/lib/stores/page-view-sync";
import { PageStore } from "@/lib/stores/stores";

interface DownloadQueue_PageStore {
  // How many cards each tab shows on a page
  queuePerPage: number;
  setQueuePerPage: (perPage: number) => void;
  failedPerPage: number;
  setFailedPerPage: (perPage: number) => void;

  // Hydrate and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useDownloadQueuePageStore = create<DownloadQueue_PageStore>()(
  persist(
    (set) => ({
      queuePerPage: 20,
      setQueuePerPage: (queuePerPage) => set({ queuePerPage }),
      failedPerPage: 20,
      setFailedPerPage: (failedPerPage) => set({ failedPerPage }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
      clear: () => set({ queuePerPage: 20, failedPerPage: 20 }),
    }),
    {
      name: "DownloadQueue",
      storage: PageStore,
      partialize: (state) => ({
        queuePerPage: state.queuePerPage,
        failedPerPage: state.failedPerPage,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);

// The options are the same on every device
syncPageView("download_queue", useDownloadQueuePageStore, ["queuePerPage", "failedPerPage"]);
