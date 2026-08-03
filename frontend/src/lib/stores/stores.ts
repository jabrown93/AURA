import { createIdbStorage } from "@/lib/stores/idb-storage";

// Separate IndexedDB databases (not just separate object stores in one DB) --
// see createIdbStorage's doc comment for why sharing one `dbName` is unsafe.
// The third argument is the object store name localforage used in the old
// shared "aura" database, so existing users' data is migrated in rather than
// silently dropped -- see migrateLegacyEntries in idb-storage.ts.
export const PageStore = createIdbStorage("aura-page-store", "PageStores", "PageStores");

export const GlobalStore = createIdbStorage("aura-global-store", "GlobalStores", "GlobalStores");
