# CLAUDE.md (backend)

## Commands

- `go build .` or `go run .` — **requires `CGO_ENABLED=1`** (SQLite via `mattn/go-sqlite3` needs cgo). Reads/writes config at `/config` default; override with `CONFIG_PATH=<dir>` for local runs.
- `sh generate_go_docs.sh` — regenerates Swagger docs into `api-docs/` via `swag init`. Run after changing routes/handlers/models; pre-commit hook does automatically for swagger-relevant changes.

## Architecture

### Staged startup with hot router swap (`main.go` + `startup.go`)
API serves *immediately* with onboarding-only routes; background goroutine runs init pipeline. Active router held in `atomic.Value` (`activeHandler`), swapped atomically once app ready. Pipeline three stages:
1. **Bootstrap** — load `config.yaml`, validate.
2. **PreFlight** — test media-server connection, validate MediUX token (sets `config.MediaServerValid` / `config.MediuxValid`).
3. **Warmup** — preload in-memory caches, init DB + run migrations, VACUUM, register cron jobs, start Plex WebSocket listener.

Config missing/invalid: router stays on onboarding routes. `routing.OnboardingComplete` = callback fired when user finishes onboarding through UI; re-runs preflight/warmup, swaps to full routes. `config.AppLoadingStep` = global string UI polls for boot progress.

### Routing (`routing/`)
`chi/v5`. `routes.go#AddRoutes` gates everything: `config.Loaded && config.Valid` false means **only** onboarding routes. Otherwise mounts full `/api` tree. Auth model:
- **Public**: `/api/login`, `/api/search`, `/api/sonarr/webhook`.
- **Protected**: everything else, behind `jwtauth.Verifier` + `middleware.Authenticator` (JWT HS256, `go-chi/jwtauth`). Passwords hashed with argon2id.

Each route group delegates to sibling handler package (`routing/config`, `routing/download`, `routing/mediaserver`, etc.). HTTP response helpers in `utils/httpx`.

### Media-server abstraction (`mediaserver/`)
`MediaServerInterface` (in `mediaserver.go`) = seam over three servers. Implementations: `mediaserver/plex` and `mediaserver/ej` (shared Emby/Jellyfin). Dispatch = `switch cfg.Type` on `"Plex"` / `"Jellyfin","Emby"`. Some capabilities Plex-exclusive (labels, ratings, event listener). **Adding media-server operation: add to interface, implement in both `plex` and `ej`.** `sonarr-radarr/` follows same interface pattern (`SonarrRadarrInterface`, `SonarrApp`/`RadarrApp`).

### MediUX client (`mediux/`)
Talks to MediUX over **GraphQL**. Queries stored as `.graphql` files (`gen_*.graphql`), executed through `make_request.go`. Handles image download/URL resolution, preloading users/items-with-sets into cache.

### Caching (`cache/`)
In-memory stores populated during warmup, refreshed by cron: `LibraryStore` (sections + items), `CollectionsStore`, plus MediUX items/users. Handlers read from these, not media server per request.

### Background jobs (`jobs/`)
`robfig/cron/v3`. Always-on jobs (download queue, refresh media items/collections, refresh MediUX users, check MediUX site link, check media-item changes, handle temp-ignored items) plus configurable auto-download job (cron from config). `cron.go` owns scheduler + job-ID bookkeeping.

### Database (`database/`)
SQLite. Files prefixed `sqlite_*` by concern. Schema evolution via **hand-written numbered migrations** in `database/migration/` (`sqlite_migration_vN_vN+1.go`), run during warmup for existing DBs. DB tracks saved image sets, ignored items, auto-download selections, schema version row.

### Config (`config/`)
Single `Config` struct (`config.go`) serialized to `/config/config.yaml`. Split across `load.go`, `save.go`, `validate.go`, `defaults.go`, and `sanitize.go`/`masking.go` (latter redact secrets before logging/returning config). Global `config.Current` holds live config plus status flags (`Loaded`, `Valid`, `MediaServerValid`, `MediuxValid`, `AppFullyLoaded`).

> Adding notification template touches **7 files** — see checklist comment on `Config_NotificationTemplate` in `config/config.go` (defaults, template_variables, validate, routing/config/update, routing/validation/notification, utils/variable_filler, frontend settings component).

### Logging & error convention (`logging/`)
Structured logging via `zerolog`. **Functions return `logging.LogErrorInfo` (struct), not Go `error`.** Callers check `Err.Message != ""`, not `err != nil`. Work wrapped in "logging context" with nested actions/sub-actions (`ld.AddAction`, `AddSubActionToContext`, `logAction.Complete()`, `logAction.SetError(...)`). Follow this pattern in new backend code, not idiomatic `error` returns.