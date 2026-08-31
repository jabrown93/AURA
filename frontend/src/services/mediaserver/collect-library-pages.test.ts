import assert from "node:assert/strict";
import test from "node:test";

import {
  beginLibraryMembershipFetch,
  buildLibraryItemsParams,
  collectLibraryPages,
} from "./collect-library-pages.ts";

test("collectLibraryPages fetches complete on-demand membership for SetFileCounts", async () => {
  const requestedPages: number[] = [];
  const allItems = Array.from({ length: 2001 }, (_, index) => index);

  const items = await collectLibraryPages(async (pageNumber) => {
    requestedPages.push(pageNumber);
    const start = (pageNumber - 1) * 1000;
    return { items: allItems.slice(start, start + 1000), totalItems: allItems.length };
  });

  assert.deepEqual(requestedPages, [1, 2, 3]);
  assert.deepEqual(items, allItems);
});

test("home page query remains page-scoped on page 2", () => {
  const params = buildLibraryItemsParams({
    libraryTitles: ["Movies"],
    searchTitle: "",
    searchLibrary: "",
    searchID: "",
    searchYear: 0,
    filterInDB: "",
    filterIgnored: "",
    filterHasSets: "",
    sortOption: "title",
    sortOrder: "asc",
    pageNumber: 2,
    itemsPerPage: 20,
  });

  assert.equal(params.page_number, 2);
  assert.equal(params.items_per_page, 20);
  assert.deepEqual(params.library_titles, ["Movies"]);
});

test("empty creator membership is fetched once and terminates", () => {
  const attemptedLibraryTitles = new Set<string>();

  assert.equal(beginLibraryMembershipFetch(attemptedLibraryTitles, "Empty", 0), true);
  assert.equal(beginLibraryMembershipFetch(attemptedLibraryTitles, "Empty", 0), false);
  assert.equal(beginLibraryMembershipFetch(attemptedLibraryTitles, "Loaded", 2), false);
  assert.deepEqual([...attemptedLibraryTitles], ["Empty"]);
});
