# CLAUDE.md (frontend)

## Commands

- `npm run dev` — dev server :3000 (version from `../VERSION.txt`, proxies `/api/*` to `localhost:8888`)
- `npm run build` — production build (Next standalone output)
- `npm run lint` / `npm run lint:fix` — ESLint (pre-commit uses `--max-warnings=0`)
- `npm run typecheck` — `tsc --noEmit`
- `npm run format` / `npm run format:check` — Prettier (import-sorting plugin)

No frontend test runner.

## Architecture

- **API access** (`src/services/`): one module per backend area (`auth`, `config`, `database`, `downloads`, `images`, `jobs`, `mediaserver`, `mediux`, `search`, ...), all on `services/api-client.ts` (axios, `baseURL: /api`). Client injects JWT from `localStorage["aura-auth-token"]` as Bearer header, redirects to `/login` on 401. In dev, `/api/*` rewritten to `http://localhost:8888` by `next.config.ts`.
- **State** (`src/lib/stores/`): Zustand. `global-store-*` = cross-app state (library sections, media, collections, poster sets, onboarding, user preferences); `page-store-*` = per-page state. `clear-all-stores.ts` resets on logout.
- `@/*` path alias → `src/*`. Shared helpers in `src/helper/`, types in `src/types/` (mirrors backend models).