import assert from "node:assert/strict";
import test from "node:test";

import { updateArtworkVersion } from "./artwork-version.ts";

test("updateArtworkVersion applies returned version without changing the media timestamp", () => {
  const item = { updated_at: 10, artwork_version: 100 };
  let appliedItem: typeof item | undefined;

  assert.equal(
    updateArtworkVersion(item, 101, (updatedItem) => {
      appliedItem = updatedItem;
    }),
    item
  );
  assert.equal(item.updated_at, 10);
  assert.equal(item.artwork_version, 101);
  assert.equal(appliedItem, item);
});

test("updateArtworkVersion ignores missing and stale versions", () => {
  const item = { updated_at: 10, artwork_version: 100 };

  updateArtworkVersion(item, undefined);
  updateArtworkVersion(item, 99);

  assert.equal(item.updated_at, 10);
  assert.equal(item.artwork_version, 100);
});
