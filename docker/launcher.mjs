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
import { readFileSync } from "node:fs";
import { request as httpRequest } from "node:http";
import { createServer as createHttpsServer } from "node:https";

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

/**
 * TLS-terminating reverse proxy in front of the Next.js UI. Next's standalone
 * server.js cannot serve HTTPS itself, and the shellless runtime image has no
 * nginx/caddy to lean on, so the supervisor terminates TLS with node's builtin
 * https module and forwards to the UI's plain-HTTP port on loopback.
 *
 * The Go backend is NOT proxied here: it terminates TLS natively on its own
 * HTTPS port (8443) from the same cert/key env vars.
 *
 * No Upgrade/WebSocket handling: no UI-facing WebSocket endpoints exist today.
 * @param {number} listenPort
 * @param {number} targetPort
 * @param {{cert: Buffer, key: Buffer}} tlsOptions
 */
// RFC 9110 hop-by-hop headers: they describe the client->proxy connection and
// must not be forwarded to the upstream (node manages its own connection
// semantics and picks chunked encoding itself when the body has no length).
const HOP_BY_HOP_HEADERS = [
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
];

function startTlsProxy(listenPort, targetPort, tlsOptions) {
  const server = createHttpsServer(tlsOptions, (req, res) => {
    const headers = { ...req.headers };
    for (const h of HOP_BY_HOP_HEADERS) delete headers[h];
    headers["x-forwarded-proto"] = "https";
    // Deliberately overwrite rather than append: this proxy is the outermost
    // TLS endpoint, so any inbound x-forwarded-for is client-supplied and
    // spoofable.
    headers["x-forwarded-for"] = req.socket.remoteAddress ?? "";
    const upstream = httpRequest(
      {
        host: "127.0.0.1",
        port: targetPort,
        path: req.url,
        method: req.method,
        headers,
      },
      (upRes) => {
        res.writeHead(upRes.statusCode ?? 502, upRes.headers);
        upRes.pipe(res);
      },
    );
    upstream.on("error", (err) => {
      console.error(`[launcher] https proxy upstream error: ${err.message}`);
      if (!res.headersSent) res.writeHead(502);
      res.end();
    });
    req.pipe(upstream);
  });

  server.on("error", (err) => {
    console.error(`[launcher] https listener :${listenPort} failed: ${err.message}`);
    shutdown(1);
  });

  server.listen(listenPort, () => {
    console.log(`[launcher] https UI listening on :${listenPort} -> :${targetPort}`);
  });
}

/**
 * Read TLS_CERT_FILE/TLS_KEY_FILE and start the HTTPS UI proxy when both are
 * set. Half-configured TLS or unreadable files are fatal: silently serving
 * HTTP only would defeat the point of configuring HTTPS. (The Go backend
 * applies the same rule for its own listener.)
 */
function startTlsIfConfigured() {
  const certFile = process.env.TLS_CERT_FILE;
  const keyFile = process.env.TLS_KEY_FILE;
  if (!certFile && !keyFile) return;
  if (!certFile || !keyFile) {
    console.error(
      "[launcher] TLS_CERT_FILE and TLS_KEY_FILE must both be set to enable HTTPS",
    );
    shutdown(1);
    return;
  }
  // createServer also throws synchronously on malformed or mismatched
  // cert/key PEM, so it shares the read's catch instead of crashing the
  // supervisor with an unhandled exception.
  try {
    const tlsOptions = { cert: readFileSync(certFile), key: readFileSync(keyFile) };
    startTlsProxy(3443, 3000, tlsOptions);
  } catch (err) {
    console.error(`[launcher] failed to load TLS cert/key: ${err.message}`);
    shutdown(1);
  }
}

process.on("SIGTERM", () => shutdown(0));
process.on("SIGINT", () => shutdown(0));

start("backend", "/app/main", []);
start("frontend", process.execPath, ["/app/server.js"]);
startTlsIfConfigured();
