import assert from "node:assert/strict";
import test from "node:test";

import { updateArtworkVersion } from "./artwork-version.ts";

test("updateArtworkVersion applies returned version to in-memory item", () => {
  const item = { updated_at: 10 };

  assert.equal(updateArtworkVersion(item, 11), item);
  assert.equal(item.updated_at, 11);
});

test("updateArtworkVersion ignores missing and stale versions", () => {
  const item = { updated_at: 10 };

  updateArtworkVersion(item, undefined);
  updateArtworkVersion(item, 9);

  assert.equal(item.updated_at, 10);
});
