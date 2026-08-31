# Contributing to AURA

AURA can be built and tested without a MediUX token or media server. Running the
complete application requires both services; see [External services](#external-services).

## Prerequisites

- Go 1.26.0 (from `backend/go.mod`)
- A C compiler and `CGO_ENABLED=1` for the SQLite driver
- Node.js 24.20.0 (from `.nvmrc`) and npm
- Git

## Install dependencies

From a fresh clone:

```sh
git clone https://github.com/jabrown93/AURA.git aura
cd aura/backend
go mod download
cd ../frontend
npm ci
```

Dependency installation may access public Go and npm registries. It does not
need AURA credentials, a MediUX token, or a media server.

## Build and test without external services

Backend package initialization writes configuration and logs under `/config` by
default. Set `CONFIG_PATH=<dir>` to a writable local directory for tests and
local runs. AURA uses SQLite through `mattn/go-sqlite3`, so backend builds and
tests require cgo and a working C compiler.

```sh
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/backend"
CONFIG_PATH=$(mktemp -d)
export CONFIG_PATH CGO_ENABLED=1
gofmt -l .
go test ./...
go vet ./...
go build ./...
```

`gofmt -l .` must print nothing. Format every file it lists before opening a
pull request.

Frontend has no test runner. Its focused checks are:

```sh
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/frontend"
npm run lint
npm run typecheck
npm run build
```

After dependencies are installed, contributors can validate these capabilities
without MediUX or Plex, Emby, or Jellyfin access:

- Backend formatting, compilation, vetting, and unit tests.
- Frontend linting, type checking, and production builds.
- Frontend onboarding UI through the local development servers below.

## Run local development servers

Start backend and frontend in separate terminals from repository root:

```sh
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/backend"
CONFIG_PATH=$(mktemp -d)
export CONFIG_PATH CGO_ENABLED=1
go run .
```

```sh
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/frontend"
npm run dev
```

Frontend runs at `http://localhost:3000`, proxies `/api/*` to backend at
`http://localhost:8888`, and can show onboarding while configuration is
incomplete. This is not a full offline application: backend activates full
routes only after preflight successfully connects to both configured external
services.

## External services

Full onboarding, library browsing, artwork retrieval/application, and other
integration flows require:

- A reachable Plex, Emby, or Jellyfin server, its API token, and a real media
  library.
- A valid MediUX API token and network access to MediUX. Token access is
  currently requested through the [AURA Discord](https://discord.gg/YAKzwKPwyw).

No mocks or fixtures currently replace these services.

## Before opening a pull request

Run checks relevant to changed code. Use a
[Conventional Commit](https://www.conventionalcommits.org/) message; repository
hooks in `.githooks/` can be enabled with:

```sh
git config core.hooksPath .githooks
```
