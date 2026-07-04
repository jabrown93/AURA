# CI / GitHub Actions

This fork's workflows follow the same conventions as the rest of `jabrown93/*`:
actions pinned by commit SHA (Renovate keeps them current via
`renovate.json` → `jabrown93/.github:renovate-config`), least-privilege
`permissions`, and a split between untrusted hosted builds and privileged
in-cluster jobs.

## Workflows

| Workflow | Trigger | What it does | Needs infra? |
|---|---|---|---|
| `ci.yml` | PR → `main`, push → `main`/`renovate/**` | Backend `go build`/`vet`/`test` + gofmt gate; frontend `npm ci`/`lint`/`build`. | No |
| `codeql.yml` | push/PR → `main`/`beta`, weekly | CodeQL for `go` and `javascript-typescript` (build-mode `none`) via the reusable `jabrown93/.github` workflow. | No |
| `release.yml` | push → `main`/`beta`, weekly (Mon 09:00 UTC), manual | semantic-release computes the next version, updates `VERSION.txt`/`version.json`/`frontend/public/CHANGELOG.md`, tags + creates a GitHub Release, then builds the multi-arch image **once** and pushes `:latest`+`:v<version>` (main) or `:beta`+`:v<version>-beta.N` (beta), plus a rolling `:edge` on every `main` push. SBOM + provenance, cosign keyless sign. | No |
| `dt-sbom.yml` | push → `main`, manual | syft SBOM (Go + npm) → upload to Dependency-Track (`isLatest`). | **Yes** |
| `pr-license-check.yml` | PR → `main` (same-repo) | Untrusted producer: build SBOM, upload as artifact. No secrets. | No |
| `pr-license-comment.yml` | `workflow_run` of the check | Trusted consumer: upload PR SBOM to DT, post advisory license comment. | **Yes** |
| `jekyll-gh-pages.yml` | push → `main` (`docs/**`), manual | Publish `docs/` to GitHub Pages. | Pages setting |

## Versioning

Versions are automated with [semantic-release](https://semantic-release.gitbook.io/)
(config in `.releaserc.js`) from [Conventional Commits](https://www.conventionalcommits.org/):

- **`main`** — `feat` → minor, `fix`/`perf` → patch, `!`/`BREAKING CHANGE` → major.
  Each release bumps `VERSION.txt`/`version.json`, prepends `frontend/public/CHANGELOG.md`
  (surfaced as in-app release notes), tags `v<x.y.z>`, and creates a GitHub Release.
- **`beta`** — prereleases (`v<x.y.z>-beta.N`), published as `:beta`.
- **Dependency bumps** (`chore(deps)`/`build(deps)`) do **not** release on push; the
  weekly Monday run sets `RELEASE_DEPS=true` and rolls them into one patch release.
- **`:edge`** — every push to `main` publishes a rolling `ghcr.io/<owner>/aura:edge`
  (and `:edge-<sha>`), independent of releases.

This is the fork's own version line (first release `v1.0.0`); the in-app update check
and version badge point at `jabrown93/AURA`, not upstream.

## Images

Images publish to **`ghcr.io/<owner>/aura`** using the built-in `GITHUB_TOKEN`
(`packages: write`) — no registry secrets to configure. They carry a CycloneDX
SBOM + max provenance and are keyless-signed (Fulcio/rekor) by digest so the
homelab Kyverno `verify-ghcr-images` policy can verify them.

## Homelab prerequisites (Dependency-Track workflows)

`dt-sbom.yml` and `pr-license-comment.yml` run their privileged half on an
in-cluster ARC runner and pull the DT API key from OpenBao via GitHub OIDC.
They stay red until this infra exists:

1. **ARC runner set `arc-oss-aura`** — a repo-scoped Actions Runner Controller
   set for `jabrown93/AURA`, egress-locked to the in-cluster OpenBao and
   Dependency-Track Services.
2. **OpenBao JWT roles** under the `github-actions` mount, bound to this repo:
   - `dt-sbom-upload` — used by `dt-sbom.yml`.
   - `dt-pr-license` — used by `pr-license-comment.yml`; bind it to
     `job_workflow_ref` `jabrown93/AURA/.github/workflows/pr-license-comment.yml@refs/heads/main`
     so a PR that rewrites the producer cannot mint the key.
   Both map `secret/data/ci/dependency-track/ci-api-key`.
3. **Dependency-Track** — reachable at
   `dependency-track-api-server.dependency-track.svc.cluster.local:8080`; the CI
   key's **Automation** team needs `BOM_UPLOAD`, `VIEW_PORTFOLIO`, and
   `VIEW_POLICY_VIOLATION`. Projects are auto-created (`github.com/jabrown93/AURA`,
   version `<sha>` for main and `pr-<n>` for PRs).

See `github.com/jabrown93/homelab → docs/dependency-track.md` for the shared
setup these mirror.

## Notes

- **GitHub Pages** (`jekyll-gh-pages.yml`) fails until Pages is enabled on the
  fork: Settings → Pages → Build and deployment → Source = "GitHub Actions".
  If you don't want to publish docs from the fork, delete that workflow instead.
- The backend has no Go tests yet; `go test ./...` is a no-op gate that will
  start enforcing as tests land.
