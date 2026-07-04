# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**aura** (Automated Utility for Retrieval of Assets) is a tool for browsing MediUX image sets and applying them to a media server (Plex, Emby, or Jellyfin). It ships as a single Docker image running two processes: a Go REST API backend (`backend/`, port 8888) and a Next.js web UI (`frontend/`, port 3000). There is no separate database service — state lives in a local SQLite file under the config directory.

## Commands

There is no Makefile. Run tooling directly from each subproject.

### Frontend (`cd frontend`)
- `npm run dev` — dev server on :3000 (reads version from `../VERSION.txt`, proxies `/api/*` to `localhost:8888`)
- `npm run build` — production build (Next standalone output)
- `npm run lint` / `npm run lint:fix` — ESLint (pre-commit runs with `--max-warnings=0`)
- `npm run typecheck` — `tsc --noEmit`
- `npm run format` / `npm run format:check` — Prettier (import-sorting plugin enabled)

There is no frontend test runner configured.

### Backend (`cd backend`)
- `go build .` or `go run .` — **requires `CGO_ENABLED=1`** (SQLite via `mattn/go-sqlite3` needs cgo). Reads/writes config at `/config` by default; override with `CONFIG_PATH=<dir>` for local runs.
- `sh generate_go_docs.sh` — regenerates Swagger docs into `api-docs/` via `swag init`. Run this after changing routes/handlers/models; the pre-commit hook does it automatically for swagger-relevant changes.

There are no Go tests in this repo.

### Full stack
- `docker compose up` (see `docker-compose.yml`) or build the multi-stage `Dockerfile` with `--build-arg APP_VERSION=$(cat VERSION.txt)`.

### Git hooks (enable manually)
Hooks live in `.githooks/` but are **not** wired up by default. Enable with `git config core.hooksPath .githooks`. They enforce:
- **commit-msg**: Conventional Commits (`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`, optional scope/`!`). Required for all commits here.
- **pre-commit**: frontend lint + typecheck + format when `frontend/` changes; regenerates Swagger docs when swagger-relevant backend files change.

## Backend architecture

### Staged startup with hot router swap (`main.go` + `startup.go`)
The API starts serving *immediately* with onboarding-only routes, while a background goroutine runs the init pipeline. The active router is held in an `atomic.Value` (`activeHandler`) and swapped in atomically once the app is ready. The pipeline has three stages:
1. **Bootstrap** — load `config.yaml`, validate it.
2. **PreFlight** — test media-server connection, validate MediUX token (sets `config.MediaServerValid` / `config.MediuxValid`).
3. **Warmup** — preload in-memory caches, init DB + run migrations, VACUUM, register cron jobs, start the Plex WebSocket listener.

If config is missing/invalid the router stays on onboarding routes. `routing.OnboardingComplete` is a callback fired when the user finishes onboarding through the UI; it re-runs preflight/warmup and swaps to full routes. `config.AppLoadingStep` is a global string the UI polls to show boot progress.

### Routing (`routing/`)
`chi/v5`. `routes.go#AddRoutes` gates everything: if `config.Loaded && config.Valid` is false it registers **only** onboarding routes. Otherwise it mounts the full `/api` tree. Auth model:
- **Public**: `/api/login`, `/api/search`, `/api/sonarr/webhook`.
- **Protected**: everything else, behind `jwtauth.Verifier` + `middleware.Authenticator` (JWT HS256, `go-chi/jwtauth`). Passwords are hashed with argon2id.

Each route group delegates to a sibling handler package (`routing/config`, `routing/download`, `routing/mediaserver`, etc.). HTTP response helpers live in `utils/httpx`.

### Media-server abstraction (`mediaserver/`)
`MediaServerInterface` (in `mediaserver.go`) is the seam over the three servers. Implementations: `mediaserver/plex` and `mediaserver/ej` (shared Emby/Jellyfin). Dispatch is a `switch cfg.Type` on `"Plex"` / `"Jellyfin","Emby"`. Some capabilities are Plex-exclusive (labels, ratings, event listener). **When adding a media-server operation, add it to the interface and implement it in both `plex` and `ej`.** `sonarr-radarr/` follows the same interface pattern (`SonarrRadarrInterface`, `SonarrApp`/`RadarrApp`).

### MediUX client (`mediux/`)
Talks to MediUX over **GraphQL**. Queries are stored as `.graphql` files (`gen_*.graphql`) and executed through `make_request.go`. Handles image download/URL resolution and preloading users/items-with-sets into cache.

### Caching (`cache/`)
In-memory stores populated during warmup and refreshed by cron: `LibraryStore` (sections + items), `CollectionsStore`, plus MediUX items/users. Handlers read from these rather than hitting the media server on every request.

### Background jobs (`jobs/`)
`robfig/cron/v3`. Always-on jobs (download queue, refresh media items/collections, refresh MediUX users, check MediUX site link, check for media-item changes, handle temp-ignored items) plus a configurable auto-download job (cron from config). `cron.go` owns the scheduler and job-ID bookkeeping.

### Database (`database/`)
SQLite. Files are prefixed `sqlite_*` by concern. Schema evolution is via **hand-written numbered migrations** in `database/migration/` (`sqlite_migration_vN_vN+1.go`), run during warmup for existing DBs. The DB tracks saved image sets, ignored items, auto-download selections, and a schema version row.

### Config (`config/`)
A single `Config` struct (`config.go`) serialized to `/config/config.yaml`. Split across `load.go`, `save.go`, `validate.go`, `defaults.go`, and `sanitize.go`/`masking.go` (the latter redact secrets before logging/returning config). Global `config.Current` holds the live config plus a set of status flags (`Loaded`, `Valid`, `MediaServerValid`, `MediuxValid`, `AppFullyLoaded`).

> Adding a notification template touches **7 files** — see the checklist comment on `Config_NotificationTemplate` in `config/config.go` (defaults, template_variables, validate, routing/config/update, routing/validation/notification, utils/variable_filler, and the frontend settings component).

### Logging & error convention (`logging/`)
Structured logging via `zerolog`. **Functions return `logging.LogErrorInfo` (a struct), not Go `error`.** Callers check `Err.Message != ""` rather than `err != nil`. Work is wrapped in a "logging context" with nested actions/sub-actions (`ld.AddAction`, `AddSubActionToContext`, `logAction.Complete()`, `logAction.SetError(...)`). Follow this pattern in new backend code instead of idiomatic `error` returns.

## Frontend architecture

Next.js **App Router** (`src/app/`), React 19, TypeScript, Tailwind v4, Radix/shadcn UI (`components/ui`), `next-themes`. Pages map to features: `onboarding`, `settings`, `collections`, `collection-item`, `media-item`, `sets`, `saved-sets`, `download-queue`, `jobs`, `logs`, `login`, `user`, `change-log`.

- **API access** (`src/services/`): one module per backend area (`auth`, `config`, `database`, `downloads`, `images`, `jobs`, `mediaserver`, `mediux`, `search`, ...), all built on `services/api-client.ts` (axios, `baseURL: /api`). The client injects the JWT from `localStorage["aura-auth-token"]` as a Bearer header and redirects to `/login` on 401. In dev, `/api/*` is rewritten to `http://localhost:8888` by `next.config.ts`.
- **State** (`src/lib/stores/`): Zustand. `global-store-*` = cross-app state (library sections, media, collections, poster sets, onboarding, user preferences); `page-store-*` = per-page state. `clear-all-stores.ts` resets on logout.
- `@/*` path alias → `src/*`. Shared helpers in `src/helper/`, types in `src/types/` (mirrors backend models).

## Release & CI

- Version is a single source: `VERSION.txt` (e.g. `v0.9.100`), mirrored in `version.json`. It's baked into both binaries at build time (`-X main.APP_VERSION` for Go; `NEXT_PUBLIC_APP_VERSION` for Next).
- `.github/workflows/aura.yml` (stable) and `aura-beta.yml` build/push multi-arch Docker images to GHCR + Docker Hub on push to `master` touching `backend/**`, `frontend/**`, or `VERSION.txt`.
- Version suffixed `dev` enables backend dev-mode logging (`main.go#init`).
- Docs are a Jekyll site under `docs/` published via `jekyll-gh-pages.yml`.
