import { useLibrarySectionsStore } from "@/lib/stores/global-store-library-sections";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { usePosterSetsStore } from "@/lib/stores/global-store-poster-sets";
// Global Stores
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";
// Page Stores
import { useHomePageStore } from "@/lib/stores/page-store-home";
import { useMediaPageStore } from "@/lib/stores/page-store-media";
import { useSavedSetsPageStore } from "@/lib/stores/page-store-saved-sets";
import { useUserPageStore } from "@/lib/stores/page-store-user";
import { GlobalStore, PageStore } from "@/lib/stores/stores";

/**
 * Clears all in‑memory zustand store slices (calls each store's clear()).
 * If options.deep === true, also wipes the underlying persisted PageStore + GlobalStore (IndexedDB),
 * removing all persisted keys (fresh start).
 */
export const ClearAllStores = async (options?: { deep?: boolean }) => {
  // 1. Clear each store's state (in memory + persisted slice contents).
  // Page stores
  useHomePageStore.getState().clear();
  useMediaPageStore.getState().clear();
  useSavedSetsPageStore.getState().clear();
  useUserPageStore.getState().clear();

  // Global stores
  useSearchQueryStore.getState().clear();
  usePosterSetsStore.getState().clear();
  useOnboardingStore.getState().clear();
  useMediaStore.getState().clear();
  useLibrarySectionsStore.getState().clear();
  useUserPreferencesStore.getState().clear();

  // 2. Optional deep clear: remove every persisted entry (including any future stores).
  if (options?.deep) {
    await Promise.all([PageStore.clear().catch(() => {}), GlobalStore.clear().catch(() => {})]);
  }
};

/**
 * Convenience helper: full deep reset (state + persisted storage).
 */
export const DeepResetAllStores = () => ClearAllStores({ deep: true });
