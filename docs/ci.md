---
layout: default
title: "CI / GitHub Actions"
nav_order: 7
---

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
| `version-release.yml` | push → `main`/`beta`, weekly (Mon 09:00 UTC), manual | semantic-release computes the next version, updates `VERSION.txt`/`version.json`/`frontend/public/CHANGELOG.md`, tags + creates a GitHub Release. Builds nothing — the tag it pushes triggers `release.yml`. Also resyncs `beta` to `main` after a stable release. | No |
| `release.yml` | GitHub Release published, manual (`publish_tag`) | Builds the multi-arch image for that release's tag and pushes `:v<version>`+`:latest` (stable) or `:v<version>-beta.N`+`:beta` (prerelease). SPDX + CycloneDX SBOMs, provenance, cosign keyless sign. | No |
| `edge.yml` | push → `main` | Builds the rolling `:edge`+`:edge-<sha>` image from main's tip, independent of releases. SPDX + CycloneDX SBOMs, provenance, cosign keyless sign. | No |
| `fossa.yml` | push → `main`, manual | Advisory licence + dependency scan via the shared `jabrown93/ci` `fossa` action. | `FOSSA_API_KEY` |
| `jekyll-gh-pages.yml` | push → `main` (`docs/**`), manual | Publish `docs/` to GitHub Pages. | Pages setting |

## Versioning

Versions are automated with [semantic-release](https://semantic-release.gitbook.io/)
(config in `.releaserc.js`) from [Conventional Commits](https://www.conventionalcommits.org/).
semantic-release and its plugins are pinned in the **root `package.json`** — CI-only
tooling, no runtime JS lives at the repo root — and run via `npm ci && npx
semantic-release`, so Renovate updates them like any other dependency:

- **`main`** — `feat` → minor, `fix`/`perf` → patch, `!`/`BREAKING CHANGE` → major.
  Each release bumps `VERSION.txt`/`version.json`, prepends `frontend/public/CHANGELOG.md`
  (surfaced as in-app release notes), tags `v<x.y.z>`, and creates a GitHub Release.
- **`beta`** — prereleases (`v<x.y.z>-beta.N`), published as `:beta`.
- **Dependency bumps** (`chore(deps)`/`build(deps)`) do **not** release on push; the
  weekly Monday run sets `RELEASE_DEPS=true` and rolls them into one patch release.
- **`:edge`** — every push to `main` publishes a rolling `ghcr.io/<owner>/aura:edge`
  (and `:edge-<sha>`), independent of releases.

### Versioning and image building are separate workflows

`version-release.yml` only computes and tags; the **GitHub Release** it publishes is
what triggers `release.yml` to build the image. The split means an image-build failure
can be retried — re-run it, or dispatch `release.yml` with `publish_tag` for a tag whose
run has aged out of GitHub's 30-day re-run window — without semantic-release
re-evaluating whether there is anything new to release.

`release.yml` deliberately triggers on `release: published` rather than the more obvious
`push: tags`, and this must not be "simplified" back. semantic-release tags its own
`chore(release): … [skip ci]` bump commit, and GitHub's skip-ci handling applies to
*any* push event — [tag pushes included][skip-ci-tags] — so a `push: tags` trigger would
be silently skipped on every release and no image would ever be built. Release events
are not filtered by commit message. (Relatedly, the Release must be created with the App
token: events from the default `GITHUB_TOKEN` never trigger workflows.) A tag pushed by
hand, with no GitHub Release, therefore builds nothing — use `publish_tag` for that.

[skip-ci-tags]: https://github.com/orgs/community/discussions/179637

Two further consequences worth knowing:

- **`:edge` trails `:latest` by one commit after a release.** semantic-release's
  version-bump commit carries `[skip ci]`, which GitHub honours natively, so `edge.yml`
  does not fire for it. That commit only touches `VERSION.txt`/`version.json`/`CHANGELOG.md`.
  The next real push to `main` brings `:edge` forward.
- **`beta` is resynced by force-push, not merge.** A squash merge of `main` → `beta`
  would not make main's release tags ancestors of `beta`, so semantic-release would keep
  computing beta's next prerelease from a superseded baseline. After each stable release
  `version-release.yml` force-pushes `beta` to main's exact tip, guarded by a
  `--force-with-lease` captured at job start (so a concurrent `beta` merge aborts the push
  rather than being silently discarded). If `beta` does not exist, the release that runs
  next creates it.

This is the fork's own version line (first release `v1.0.0`); the in-app update check
and version badge point at `jabrown93/AURA`, not upstream.

## Images

Images publish to **`ghcr.io/<owner>/aura`** using the built-in `GITHUB_TOKEN`
(`packages: write`) — no registry secrets to configure. They carry max
provenance and **both** SBOM formats as OCI referrers, and are keyless-signed
(Fulcio/rekor) by digest so the homelab Kyverno `verify-ghcr-images` policy can
verify them.

Two formats because BuildKit's `sbom: true` attestation is SPDX-only and cannot
emit CycloneDX, so a syft scan of the pushed digest is attested separately:

```sh
docker buildx imagetools inspect ghcr.io/<owner>/aura@<digest> --format '{{json .SBOM}}'   # SPDX
cosign download attestation --predicate-type cyclonedx ghcr.io/<owner>/aura@<digest>       # CycloneDX
```

The CycloneDX pass is scanned from the digest, never the tag — a floating tag
can move between push and scan. syft resolves the multi-arch index to a single
manifest, so that half describes one architecture; these images differ by
compiled arch, not package set.

## Licence scanning prerequisites (`fossa.yml`)

`FOSSA_API_KEY` must exist in this repo's Actions secrets. It is fanned out by
External Secrets from homelab, not set by hand. It is a write-scoped *push*
token, which is why the workflow runs on `push` to `main` only and never on
`pull_request`: a PR must not be able to reach it, and a fork PR would not
receive it regardless. The scan is therefore post-merge and advisory.

The workflow that preceded it — `dt-sbom.yml` plus the `pr-license-check.yml` /
`pr-license-comment.yml` pair — uploaded SBOMs to Dependency-Track from an
in-cluster ARC runner. Those never ran: the `arc-oss-aura` runner set was never
created, so the privileged half queued until GitHub expired it. Dependency-Track
is decommissioned (jabrown93/homelab#3167) and all three workflows are deleted.
Image SBOM publishing is unaffected — `release.yml`/`edge.yml` attach both
formats as OCI referrers, as above.

## Notes

- **GitHub Pages** (`jekyll-gh-pages.yml`) fails until Pages is enabled on the
  fork: Settings → Pages → Build and deployment → Source = "GitHub Actions".
  If you don't want to publish docs from the fork, delete that workflow instead.
- The backend has no Go tests yet; `go test ./...` is a no-op gate that will
  start enforcing as tests land.
