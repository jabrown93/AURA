# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repo.

## What this is

**aura** (Automated Utility for Retrieval of Assets) browses MediUX image sets, applies them to media server (Plex, Emby, or Jellyfin). Ships as single Docker image, two processes: Go REST API backend (`backend/`, port 8888) and Next.js web UI (`frontend/`, port 3000). No separate database service — state lives in local SQLite file under config directory.

## Commands

No Makefile. Run tooling directly from each subproject; see `frontend/CLAUDE.md` and `backend/CLAUDE.md` for per-side commands.

### Full stack
- `docker compose up` (see `docker-compose.yml`) or build multi-stage `Dockerfile` with `--build-arg APP_VERSION=$(cat VERSION.txt)`.

### Git hooks (enable manually)
Hooks live in `.githooks/`, **not** wired up by default. Enable with `git config core.hooksPath .githooks`. Enforce:
- **commit-msg**: Conventional Commits (`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`, optional scope/`!`). Required for all commits here.
- **pre-commit**: frontend lint + typecheck + format when `frontend/` changes; regenerates Swagger docs when swagger-relevant backend files change.

## Release & CI

- Version single source: `VERSION.txt` (e.g. `v0.9.100`), mirrored in `version.json`. Baked into both binaries at build time (`-X main.APP_VERSION` for Go; `NEXT_PUBLIC_APP_VERSION` for Next).
- `.github/workflows/aura.yml` (stable) and `aura-beta.yml` build/push multi-arch Docker images to GHCR + Docker Hub on push to `master` touching `backend/**`, `frontend/**`, or `VERSION.txt`.
- Version suffixed `dev` enables backend dev-mode logging (`main.go#init`).
- Docs: Jekyll site under `docs/`, published via `jekyll-gh-pages.yml`.