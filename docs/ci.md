# CI / GitHub Actions

This fork's workflows follow the same conventions as the rest of `jabrown93/*`:
actions pinned by commit SHA (Renovate keeps them current via
`renovate.json` → `jabrown93/.github:renovate-config`), least-privilege
`permissions`, and a split between untrusted hosted builds and privileged
in-cluster jobs.

## Workflows

| Workflow | Trigger | What it does | Needs infra? |
|---|---|---|---|
| `ci.yml` | PR → `master`, push → `master`/`renovate/**` | Backend `go build`/`vet`/`test` + gofmt gate; frontend `npm ci`/`lint`/`build`. | No |
| `codeql.yml` | push/PR → `master`/`beta`, weekly | CodeQL for `go` and `javascript-typescript` (build-mode `none`) via the reusable `jabrown93/.github` workflow. | No |
| `aura.yml` | push → `master` (paths), manual | Multi-arch build + push `ghcr.io/<owner>/aura:latest` + `:<VERSION.txt>`, SBOM + provenance, cosign keyless sign. | No |
| `aura-beta.yml` | push → `beta*` (paths), manual | Same, `:beta` + `:<VERSION>-beta`. | No |
| `dt-sbom.yml` | push → `master`, manual | syft SBOM (Go + npm) → upload to Dependency-Track (`isLatest`). | **Yes** |
| `pr-license-check.yml` | PR → `master` (same-repo) | Untrusted producer: build SBOM, upload as artifact. No secrets. | No |
| `pr-license-comment.yml` | `workflow_run` of the check | Trusted consumer: upload PR SBOM to DT, post advisory license comment. | **Yes** |
| `jekyll-gh-pages.yml` | push → `master` (`docs/**`), manual | Publish `docs/` to GitHub Pages. | Pages setting |

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
     `job_workflow_ref` `jabrown93/AURA/.github/workflows/pr-license-comment.yml@refs/heads/master`
     so a PR that rewrites the producer cannot mint the key.
   Both map `secret/data/ci/dependency-track/ci-api-key`.
3. **Dependency-Track** — reachable at
   `dependency-track-api-server.dependency-track.svc.cluster.local:8080`; the CI
   key's **Automation** team needs `BOM_UPLOAD`, `VIEW_PORTFOLIO`, and
   `VIEW_POLICY_VIOLATION`. Projects are auto-created (`github.com/jabrown93/AURA`,
   version `<sha>` for master and `pr-<n>` for PRs).

See `github.com/jabrown93/homelab → docs/dependency-track.md` for the shared
setup these mirror.

## Notes

- **GitHub Pages** (`jekyll-gh-pages.yml`) fails until Pages is enabled on the
  fork: Settings → Pages → Build and deployment → Source = "GitHub Actions".
  If you don't want to publish docs from the fork, delete that workflow instead.
- The backend has no Go tests yet; `go test ./...` is a no-op gate that will
  start enforcing as tests land.
