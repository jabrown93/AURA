import { create } from "zustand";
import { persist } from "zustand/middleware";

import { GlobalStore } from "@/lib/stores/stores";

import type { CollectionItem } from "@/types/media-and-posters/collection-item";

interface CollectionStore {
  collectionItem: CollectionItem | null;
  setCollectionItem: (collectionItem: CollectionItem | null) => void;

  clear: () => void;
  hasHydrated: boolean;
  hydrate: () => void;
}

export const useCollectionStore = create<CollectionStore>()(
  persist(
    (set) => ({
      collectionItem: null,
      setCollectionItem: (collectionItem) => set({ collectionItem }),

      clear: () => set({ collectionItem: null }),
      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
    }),
    {
      name: "CurrentCollection",
      storage: GlobalStore,
      partialize: (state) => ({
        collectionItem: state.collectionItem,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
