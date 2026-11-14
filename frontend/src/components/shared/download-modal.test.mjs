import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { URL } from "node:url";
import ts from "typescript";

// Exercise the modal's closures without introducing a React rendering test dependency.
const source = readFileSync(new URL("./download-modal.tsx", import.meta.url), "utf8");
const enqueueStart = source.indexOf("  const runAddToQueueTask =");
const finishStart = source.indexOf("  const finishQueueJob =");
assert.ok(enqueueStart >= 0 && finishStart >= 0);
const enqueue = source.slice(enqueueStart, source.indexOf("\n  };", enqueueStart) + 5);
const finish = source.slice(finishStart, source.indexOf("\n  };", finishStart) + 5);
const code = ts.transpileModule(`${enqueue}\n${finish}\nreturn { runAddToQueueTask, finishQueueJob };`, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;

for (const early of [true, false]) {
  for (const status of ["completed", "failed"]) {
    test(`${status} event ${early ? "before" : "after"} enqueue response`, async () => {
      const queueJobsRef = { current: {} };
      const terminalJobsRef = { current: {} };
      let waiting = 0;
      let task = { status: "pending", payload: {} };
      let api;
      const payload = { itemTitle: "Test", dbItem: {} };
      const create = new Function(
        "queueJobsRef",
        "terminalJobsRef",
        "setWaitingJobs",
        "updateTask",
        "setCurrentText",
        "AddItemToDownloadQueue",
        "markItemSaved",
        "STOPPED_ERROR",
        code
      );
      api = create(
        queueJobsRef,
        terminalJobsRef,
        (update) => {
          waiting = update(waiting);
        },
        (_, update) => {
          task = update(task);
        },
        () => {},
        async () => {
          if (early) api.finishQueueJob(42, status, status === "failed" ? "failed" : undefined);
          return { status: "success", data: { job_id: 42 } };
        },
        () => {},
        "stopped"
      );
      assert.equal(await api.runAddToQueueTask("task", payload), true);
      if (!early) api.finishQueueJob(42, status, status === "failed" ? "failed" : undefined);
      assert.equal(waiting, 0);
      assert.equal(task.status, status);
      assert.deepEqual(queueJobsRef.current, {});
      assert.deepEqual(terminalJobsRef.current, {});
    });
  }
}
