import assert from "node:assert/strict";
import test from "node:test";

import { getDependencyWarning } from "./app-status-warning.ts";

const status = {
  media_server_reachable: true,
  mediux_reachable: true,
  mediux_valid: true,
};

test("reports unreachable and rejected MediUX states distinctly", () => {
  assert.deepEqual(getDependencyWarning({ ...status, mediux_reachable: false, mediux_valid: false }), {
    label: "MediUX is unreachable",
    detail: "MediUX is unreachable. aura is running with reduced functionality and will reconnect automatically.",
  });
  assert.deepEqual(getDependencyWarning({ ...status, mediux_valid: false }), {
    label: "MediUX token is invalid",
    detail: "MediUX rejected configured token. Update it in settings to restore MediUX functionality.",
  });
});

test("returns no warning after recovery", () => {
  assert.equal(getDependencyWarning(status), null);
});
