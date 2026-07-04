// Process supervisor for the aura container.
//
// The runtime image (Docker Hardened Image, shellless + rootless) has no shell,
// so the two long-running processes can't be started with `sh -c "./main & node
// server.js"`. Instead node — the image's runtime — supervises both:
//
//   * the Go REST API backend  (/app/main, port 8888)
//   * the Next.js standalone UI (/app/server.js, port 3000)
//
// If either child exits, we tear the other down and exit non-zero so the
// container orchestrator (`restart: unless-stopped`) restarts the whole thing,
// rather than leaving a half-dead container serving a broken UI or a dead API.

import { spawn } from "node:child_process";

const SHUTDOWN_GRACE_MS = 10_000;

/** @type {{name: string, child: import("node:child_process").ChildProcess}[]} */
const procs = [];
let shuttingDown = false;

/**
 * Terminate every still-running child, then exit. Idempotent: the first caller
 * wins and later triggers (a second child dying, a second signal) are ignored.
 * @param {number} code
 */
function shutdown(code) {
  if (shuttingDown) return;
  shuttingDown = true;

  for (const { child } of procs) {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGTERM");
    }
  }

  // Backstop: if a child ignores SIGTERM, force it and exit anyway.
  const force = setTimeout(() => {
    for (const { child } of procs) child.kill("SIGKILL");
    process.exit(code);
  }, SHUTDOWN_GRACE_MS);
  force.unref();

  Promise.all(
    procs.map(
      ({ child }) =>
        new Promise((resolve) =>
          child.exitCode !== null || child.signalCode !== null
            ? resolve()
            : child.once("exit", resolve),
        ),
    ),
  ).then(() => process.exit(code));
}

/**
 * @param {string} name
 * @param {string} command
 * @param {string[]} args
 */
function start(name, command, args) {
  const child = spawn(command, args, { cwd: "/app", stdio: "inherit" });
  procs.push({ name, child });

  child.on("error", (err) => {
    console.error(`[launcher] failed to start ${name}: ${err.message}`);
    shutdown(1);
  });

  child.on("exit", (exitCode, signal) => {
    if (shuttingDown) return;
    console.error(
      `[launcher] ${name} exited (code=${exitCode}, signal=${signal}); stopping container`,
    );
    // Any child exiting is treated as fatal so the container restarts cleanly.
    shutdown(exitCode === 0 ? 1 : exitCode ?? 1);
  });
}

process.on("SIGTERM", () => shutdown(0));
process.on("SIGINT", () => shutdown(0));

start("backend", "/app/main", []);
start("frontend", process.execPath, ["/app/server.js"]);
