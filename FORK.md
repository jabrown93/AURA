# Fork notes

Unofficial fork of [mediux-team/AURA](https://github.com/mediux-team/AURA). This file is the
contract for keeping it close to upstream: what differs, why, and how to sync.

## Syncing upstream

`main` is squash-only with signed commits, so upstream history never lands here and git cannot
find the merge base by itself. `.upstream-sync` records the last upstream commit already merged.

```bash
scripts/sync-upstream.sh            # branch from origin/main, merge upstream/master
# resolve conflicts (policy below), run the checks, git add
scripts/sync-upstream.sh --finish   # one commit on top of origin/main; updates .upstream-sync
```

Open a PR from the branch. `.github/workflows/upstream-sync.yml` does this weekly: a clean merge
becomes a PR, a conflicting one becomes a tracking issue.

Never edit `.upstream-sync` by hand to a commit that has not actually been merged; the next sync
would silently drop everything before it.

Checks (CI runs the first and last two):

```bash
cd backend && CGO_ENABLED=1 go vet ./... && CGO_ENABLED=1 CONFIG_PATH=$(mktemp -d) go test ./...
cd frontend && npm ci && npm run lint && npm run build
```

To tell merge regressions from upstream's own failures, run the Go tests on a pristine checkout of
the target upstream commit and compare.

### Conflict policy

1. **Upstream's code shape wins.** Fork-only refactors of upstream code are not kept. Re-apply the
   fork *behavior* as the smallest patch on upstream's version, marked with a `// Fork:` comment,
   or in a `fork_*.go` / `Fork`-named file so the next conflict is obvious.
2. **Keep fork behavior listed below.** Anything not listed is upstream's call.
3. **Fork-owned paths** keep the fork version and the script restores them automatically:
   `.github/workflows/`, `Dockerfile`, `docker-entrypoint.sh`, `VERSION.txt`, `version.json`,
   `frontend/public/CHANGELOG.md`, `.releaserc.js`, `renovate.json`. Port upstream changes there
   deliberately (e.g. new Dockerfile build args).
4. **Dependencies:** `backend/go.mod` and `frontend/package.json` keep the fork's versions and
   replacements; add upstream's new dependencies, then regenerate `go.sum` (`go mod tidy`) and
   `package-lock.json` (`npm install`). Upstream imports of a replaced package are rewritten to the
   fork's replacement (table below).
5. **Schema:** fork columns are added by `applyForkSchema` in
   `backend/database/migration/fork_migrations.go`, never as a numbered migration, so upstream's
   numbering can't collide with ours again.
6. New behavioral conflicts the policy does not cover: record a decision here before merging.

## Fork differences

| Area | Behavior | Where |
| --- | --- | --- |
| Container | Shell-less, rootless DHI Node runtime; `docker/launcher.mjs` supervises both processes and forwards SIGTERM. No `HEALTHCHECK` (no `wget` in the image). | `Dockerfile`, `docker/launcher.mjs` |
| HTTPS | `TLS_CERT_FILE`/`TLS_KEY_FILE` add HTTPS listeners on 3443 (UI) and 8443 (API). | `backend/fork_startup.go`, `docker/launcher.mjs` |
| Versioning | semantic-release from Conventional Commits; the in-app changelog shows fork releases. | `.releaserc.js`, `version-release.yml` |
| Kometa | Write applied images into Kometa's asset directory, per-library subfolders, Sonarr/Radarr fallback, preload missing seasons/episodes, asset import job. Plex only. | `backend/kometa/`, `config/fork_kometa.go`, `mediaserver/fork_kometa.go`, `download/queue/fork_kometa.go`, `ConfigSectionKometa.tsx` |
| Anime | HAMA/legacy Plex GUIDs and an AniDB → TMDB mapping resolve anime items. | `backend/anidb/`, `mediaserver/plex/guids.go` |
| Startup | A media-server/MediUX outage at boot is retried in the background instead of opening setup; a MediUX outage starts degraded with a navbar warning and rebuilds caches on recovery. | `backend/fork_startup.go`, `app-status-warning.ts` |
| Jobs | A job never overlaps itself; a manual run of a busy job gets 409. | `backend/jobs/fork_overlap.go` |
| Queue | A stale rating key is re-resolved by TMDB ID and edition after a 404; Retry All on the Failed tab. | `download/queue/fork_kometa.go`, `download-history-retry-all.tsx` |
| Webhooks | Radarr webhook beside Sonarr's. | `routing/sonarr-radarr/webhook_radarr.go` |
| Security | Masked tokens and webhook headers are only restored for the URL they were saved with; CSRF Origin check on cookie-authenticated writes; OIDC `email_verified` required for the email allowlist, plus `AllowedGroups`; webhook URLs never logged; outbound requests honor the caller's context; server header/body timeouts; library images `Cache-Control: private, no-store`; the public MediUX image `asset_id` is restricted to ID characters (it names a cache file). | `config/fork_masked_token.go`, `routing/config/fork_webhook_headers.go`, `routing/middleware/csrf.go`, `routing/auth/fork_oidc_authz.go`, `utils/httpx/`, `mediux/fork_asset_id.go` |
| Dependencies | Abandoned packages replaced: `alexedwards/argon2id` → `utils/argon2id`, `gopkg.in/yaml.v3` → `go.yaml.in/yaml/v3`, `gorilla/websocket` → `coder/websocket`, `swaggo/http-swagger` → `/v2`, `blurhash` → `fast-blurhash` (`lib/blurhash.ts`), `localforage` → `idb-keyval` (`lib/stores/idb-storage.ts`), `vaul` removed. | `go.mod`, `package.json` |
| Docs | Published from this repo; extra pages for Kometa, HTTPS and CI. | `docs/` |

## Decisions

Dated so a later sync can revisit them.

- **2026-10 sync (upstream v0.9.100 `a2b98890` → v0.9.115 `273b6dab`)**
  - Sync mechanism: `.upstream-sync` marker plus `scripts/sync-upstream.sh`, as in jabrown93/CrossWatch.
  - Auth: upstream's OIDC (PKCE, several redirect URLs), API keys, optional password and setup code
    replace the fork's OIDC implementation. Kept from the fork: CSRF check, groups allowlist,
    `email_verified` gate. Dropped: `RPInitiatedLogout`, `ButtonLabel`, `Scopes`, `AllowedSubjects`.
  - Config: upstream's database-stored config. Deployments that own `config.yaml` in Git set
    `AURA_IMPORT_CONFIG=true` so it replaces the stored config on every start; the fork keeps the
    stored `UserPreferences` on that re-import unless the file sets them (`config/fork_import.go`).
  - Kometa: rebuilt on upstream's `localimage` saver. `Images.Kometa.ImportCron` became the
    `Jobs.KometaImport` job; the AniDB refresh is `Jobs.AnidbRefresh`.
  - Download queue: upstream's database queue. Dropped the fork's file-based collection queue
    (upstream queues collection images) and per-entry bulk selection (upstream has Clear Failed and
    automatic retries; the fork adds Retry All).
  - Schema: the fork's old v5→v6 (`force_preload_missing`) is rewound to v5 on first start so
    upstream's v6–v14 run, then the column is re-added and its values restored.
  - Versioning stays semantic-release; upstream's `VERSION.txt` and changelog are not taken.
  - Docs: upstream's Starlight site, published by `docs.yml`; Jekyll removed.
  - Image caching: artwork versioning (#152/#165/#174) dropped; library images are `private, no-store`.
  - Scheduler: upstream's `robfig/cron` (the fork had moved to gocron) plus the overlap guard.
  - Startup: upstream's wait-with-backoff, then the fork's background retry and degraded start.
