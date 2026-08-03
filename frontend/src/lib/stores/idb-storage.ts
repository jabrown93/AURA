import { type UseStore, createStore, del, get, keys, clear as idbClear, set } from "idb-keyval";
import type { PersistStorage, StorageValue } from "zustand/middleware";

/**
 * Zustand's `persist` storage option, plus a `clear()` method used by
 * `clear-all-stores.ts` to wipe every persisted key for this instance.
 */
export type IdbPersistStorage<S> = PersistStorage<S> & { clear: () => Promise<void> };

const LEGACY_DB_NAME = "aura";

/**
 * localforage's IndexedDB driver stores each key's value directly (structured
 * clone, no wrapper) in the object store it was created with -- the same
 * shape idb-keyval reads/writes. So migrating out of the old shared "aura"
 * database is a plain per-key copy, not a format conversion.
 */
async function readLegacyEntries(legacyStoreName: string): Promise<[IDBValidKey, unknown][]> {
  if (typeof indexedDB === "undefined") {
    return [];
  }

  return new Promise((resolve) => {
    // Opened with no explicit version, so if "aura" already exists (it was
    // created by localforage) this connects to it as-is with no
    // onupgradeneeded. `IDBFactory.databases()` would be a cleaner existence
    // check, but it isn't supported everywhere; gating on it meant the
    // migration silently no-op'd on those browsers and then permanently
    // skipped itself the moment the new store got its first write -- the
    // exact reset this migration exists to prevent. If "aura" doesn't exist,
    // this open creates an empty version-1 database as a side effect (no
    // object stores, since there's no onupgradeneeded handler below to add
    // any) -- harmless clutter for a fresh install, traded for migration
    // actually working everywhere else.
    const openReq = indexedDB.open(LEGACY_DB_NAME);
    openReq.onerror = () => resolve([]);
    openReq.onsuccess = () => {
      const db = openReq.result;
      if (!db.objectStoreNames.contains(legacyStoreName)) {
        db.close();
        resolve([]);
        return;
      }

      const tx = db.transaction(legacyStoreName, "readonly");
      const store = tx.objectStore(legacyStoreName);
      const keysReq = store.getAllKeys();
      const valuesReq = store.getAll();

      tx.oncomplete = () => {
        db.close();
        const entryKeys = keysReq.result;
        const values = valuesReq.result as unknown[];
        resolve(entryKeys.map((key, i) => [key, values[i]]));
      };
      tx.onerror = () => {
        db.close();
        resolve([]);
      };
    };
  });
}

/**
 * One-time copy of this store's data out of the old shared `localforage`
 * database, so upgrading users don't silently lose persisted state (filters,
 * saved sets, onboarding status, etc.) just because the storage backend
 * changed. Runs at most once per store: skipped entirely once the new store
 * already has any key, whether from a prior migration or a fresh write.
 */
async function migrateLegacyEntries(legacyStoreName: string, target: UseStore): Promise<void> {
  const existingKeys = await keys(target);
  if (existingKeys.length > 0) {
    return;
  }

  const legacyEntries = await readLegacyEntries(legacyStoreName);
  for (const [key, value] of legacyEntries) {
    if (typeof key === "string") {
      await set(key, value, target);
    }
  }
}

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
export function createIdbStorage<S>(
  dbName: string,
  storeName: string,
  legacyStoreName: string,
): IdbPersistStorage<S> {
  let store: UseStore | undefined;
  const getStore = (): UseStore => (store ??= createStore(dbName, storeName));

  let migration: Promise<void> | undefined;
  const ensureMigrated = (): Promise<void> =>
    (migration ??= migrateLegacyEntries(legacyStoreName, getStore()));

  return {
    getItem: async (name) => {
      await ensureMigrated();
      const value = await get<StorageValue<S>>(name, getStore());
      return value ?? null;
    },
    setItem: async (name, value) => {
      await ensureMigrated();
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
