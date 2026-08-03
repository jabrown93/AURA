import { createIdbStorage } from "@/lib/stores/idb-storage";

// Separate IndexedDB databases (not just separate object stores in one DB) --
// see createIdbStorage's doc comment for why sharing one `dbName` is unsafe.
export const PageStore = createIdbStorage("aura-page-store", "PageStores");

export const GlobalStore = createIdbStorage("aura-global-store", "GlobalStores");
