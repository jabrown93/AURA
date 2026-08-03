import { type UseStore, createStore, del, get, clear as idbClear, set } from "idb-keyval";
import type { PersistStorage, StorageValue } from "zustand/middleware";

/**
 * Zustand's `persist` storage option, plus a `clear()` method used by
 * `clear-all-stores.ts` to wipe every persisted key for this instance.
 */
export type IdbPersistStorage<S> = PersistStorage<S> & { clear: () => Promise<void> };

/**
 * Builds a zustand `persist` storage adapter backed by its own IndexedDB
 * database (via idb-keyval). The underlying `createStore` call is deferred
 * until first use: idb-keyval opens the database eagerly on `createStore()`,
 * and this module is imported by "use client" components that Next.js still
 * evaluates in Node during SSR/prerendering, where `indexedDB` is undefined.
 *
 * Each caller must use a unique `dbName` -- idb-keyval only creates
 * `storeName` in the database's `onupgradeneeded` handler, so reusing a
 * `dbName` across two `createStore` calls silently skips creating the
 * second object store.
 */
export function createIdbStorage<S>(dbName: string, storeName: string): IdbPersistStorage<S> {
  let store: UseStore | undefined;
  const getStore = (): UseStore => (store ??= createStore(dbName, storeName));

  return {
    getItem: async (name) => {
      const value = await get<StorageValue<S>>(name, getStore());
      return value ?? null;
    },
    setItem: async (name, value) => {
      await set(name, value, getStore());
    },
    removeItem: async (name) => {
      await del(name, getStore());
    },
    clear: async () => {
      await idbClear(getStore());
    },
  };
}
