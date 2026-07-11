import { create } from "zustand";
import { persist } from "zustand/middleware";

import { GlobalStore } from "@/lib/stores/stores";

import type { CollectionItem } from "@/types/media-and-posters/collection-item";

interface CollectionStore {
  collectionItem: CollectionItem | null;
  setCollectionItem: (collectionItem: CollectionItem | null) => void;

  collectionItems: CollectionItem[] | null;
  setCollectionItems: (collectionItems: CollectionItem[] | null, timestamp?: number) => void;
  timestamp?: number;
  setTimestamp: (timestamp: number) => void;

  clear: () => void;
  hasHydrated: boolean;
  hydrate: () => void;
}

export const useCollectionStore = create<CollectionStore>()(
  persist(
    (set) => ({
      collectionItem: null,
      setCollectionItem: (collectionItem) => set({ collectionItem }),

      collectionItems: null,
      setCollectionItems: (collectionItems, timestamp) =>
        set({
          collectionItems,
          timestamp: timestamp ?? Date.now(),
        }),

      timestamp: undefined,
      setTimestamp: (timestamp) => set({ timestamp }),

      clear: () => set({ collectionItem: null, collectionItems: null, timestamp: undefined }),
      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
    }),
    {
      name: "CurrentCollection",
      storage: GlobalStore,
      partialize: (state) => ({
        collectionItem: state.collectionItem,
        collectionItems: state.collectionItems,
        timestamp: state.timestamp,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
