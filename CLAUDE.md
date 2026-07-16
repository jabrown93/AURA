# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**aura** (Automated Utility for Retrieval of Assets) is a tool for browsing MediUX image sets and applying them to a media server (Plex, Emby, or Jellyfin). It ships as a single Docker image running two processes: a Go REST API backend (`backend/`, port 8888) and a Next.js web UI (`frontend/`, port 3000). There is no separate database service — state lives in a local SQLite file under the config directory.

## Commands

There is no Makefile. Run tooling directly from each subproject; see `frontend/CLAUDE.md` and `backend/CLAUDE.md` for per-side commands.

### Full stack
- `docker compose up` (see `docker-compose.yml`) or build the multi-stage `Dockerfile` with `--build-arg APP_VERSION=$(cat VERSION.txt)`.

### Git hooks (enable manually)
Hooks live in `.githooks/` but are **not** wired up by default. Enable with `git config core.hooksPath .githooks`. They enforce:
- **commit-msg**: Conventional Commits (`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`, optional scope/`!`). Required for all commits here.
- **pre-commit**: frontend lint + typecheck + format when `frontend/` changes; regenerates Swagger docs when swagger-relevant backend files change.

## Release & CI

- Version is a single source: `VERSION.txt` (e.g. `v0.9.100`), mirrored in `version.json`. It's baked into both binaries at build time (`-X main.APP_VERSION` for Go; `NEXT_PUBLIC_APP_VERSION` for Next).
- `.github/workflows/aura.yml` (stable) and `aura-beta.yml` build/push multi-arch Docker images to GHCR + Docker Hub on push to `master` touching `backend/**`, `frontend/**`, or `VERSION.txt`.
- Version suffixed `dev` enables backend dev-mode logging (`main.go#init`).
- Docs are a Jekyll site under `docs/` published via `jekyll-gh-pages.yml`.
