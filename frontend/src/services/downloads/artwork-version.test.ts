import assert from "node:assert/strict";
import test from "node:test";

import { updateArtworkVersion } from "./artwork-version.ts";

test("updateArtworkVersion applies returned version and notifies the page", () => {
  const item = { updated_at: 10 };
  let appliedItem: typeof item | undefined;

  assert.equal(
    updateArtworkVersion(item, 11, (updatedItem) => {
      appliedItem = updatedItem;
    }),
    item
  );
  assert.equal(item.updated_at, 11);
  assert.equal(appliedItem, item);
});

test("updateArtworkVersion ignores missing and stale versions", () => {
  const item = { updated_at: 10 };

  updateArtworkVersion(item, undefined);
  updateArtworkVersion(item, 9);

  assert.equal(item.updated_at, 10);
});
