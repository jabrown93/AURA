import type { StoreApi, UseBoundStore } from "zustand";

import { type PageViewPage, useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { PageView } from "@/types/config/config";

// While paused, changes aren't saved, so clearing the stores doesn't clear the saved options
let paused = false;

// withoutPageViewSync runs fn without saving the page changes it makes
export const withoutPageViewSync = (fn: () => void) => {
  paused = true;
  try {
    fn();
  } finally {
    paused = false;
  }
};

const sameView = (a: PageView | undefined, b: PageView | undefined) =>
  JSON.stringify(a ?? {}) === JSON.stringify(b ?? {});

// syncPageView keeps a page's sort and filter options in the User Preferences, so they're the same on every device.
// Changes on this device are saved, and saved options from other devices are applied.
// The page is only saved once it has loaded its local copy, so that copy doesn't replace the saved options.
export const syncPageView = <S extends { hasHydrated: boolean }>(
  page: PageViewPage,
  store: UseBoundStore<StoreApi<S>>,
  keys: (keyof S & string)[]
) => {
  if (typeof window === "undefined") return;

  const pick = (state: S): PageView => Object.fromEntries(keys.map((key) => [key, state[key]]));
  let applying = false;

  // apply shows the saved options, when the page and the User Preferences have loaded
  const apply = () => {
    const saved = useUserPreferencesStore.getState().pageViews?.[page];
    if (!saved || !store.getState().hasHydrated || !useUserPreferencesStore.getState().hasHydrated) return;
    const view = Object.fromEntries(keys.filter((key) => key in saved).map((key) => [key, saved[key]]));
    if (sameView(pick({ ...store.getState(), ...view }), pick(store.getState()))) return;
    applying = true;
    store.setState(view as Partial<S>);
    applying = false;
  };

  store.subscribe((state, previous) => {
    if (state.hasHydrated && !previous.hasHydrated) {
      apply();
      return;
    }
    if (paused || applying || !state.hasHydrated || !useUserPreferencesStore.getState().hasHydrated) return;
    const view = pick(state);
    if (sameView(view, pick(previous)) || sameView(view, useUserPreferencesStore.getState().pageViews?.[page])) return;
    useUserPreferencesStore.getState().setPageView(page, view);
  });

  useUserPreferencesStore.subscribe((state, previous) => {
    if (state.pageViews?.[page] !== previous.pageViews?.[page] || (state.hasHydrated && !previous.hasHydrated)) {
      apply();
    }
  });

  apply();
};
