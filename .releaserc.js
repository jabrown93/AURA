// semantic-release configuration.
//
// Versioning is automated from Conventional Commits:
//   * push to `main` -> stable release (feat -> minor, fix/perf -> patch, ! -> major)
//   * push to `beta` -> prerelease (vX.Y.Z-beta.N)
//
// Dependency bumps intentionally do NOT cut a release on ordinary pushes. Renovate
// labels them fix(deps) (runtime deps), chore(deps) (dev deps / lock maintenance),
// or build(deps) — fix would otherwise trigger a patch via the default rules, so it
// is explicitly suppressed here. The weekly scheduled workflow run sets
// RELEASE_DEPS=true, which promotes all accumulated dependency commits to a single
// patch release.
//
// This file is CommonJS (there is no root package.json); semantic-release loads
// it via cosmiconfig. `${...}` placeholders are expanded by semantic-release, not
// by JS — keep them inside double-quoted strings so JS does not interpolate them.

const releaseDeps = process.env.RELEASE_DEPS === "true";

// Custom rules are evaluated before commit-analyzer's defaults, and the first
// match wins — so `release: false` on fix(deps) suppresses the default fix->patch.
// chore/build already don't release by default; they only need promotion when
// RELEASE_DEPS is set.
const depReleaseRules = releaseDeps
  ? [
      { type: "fix", scope: "deps", release: "patch" },
      { type: "chore", scope: "deps", release: "patch" },
      { type: "build", scope: "deps", release: "patch" },
    ]
  : [{ type: "fix", scope: "deps", release: false }];

// GitHub rejects a Release body over 125,000 characters (HTTP 422). Normal
// automated releases are far below that, but a first release with no prior tag
// rolls up the entire history and can blow past it — which wedged v1.0.0 here.
// Cap ONLY the Release body: @semantic-release/changelog still writes the full
// notes to CHANGELOG.md. This is a Lodash template (@semantic-release/github
// v11+): `<% %>` runs JS, `<%= %>` inserts the value raw (no HTML escaping).
//
// Build the "full changelog" link from the runner's repo env so it survives a
// repo rename/transfer (falls back to the canonical URL for local dry-runs).
const repoBaseUrl =
  process.env.GITHUB_SERVER_URL && process.env.GITHUB_REPOSITORY
    ? `${process.env.GITHUB_SERVER_URL}/${process.env.GITHUB_REPOSITORY}`
    : "https://github.com/jabrown93/AURA";
const RELEASE_BODY_MAX = 120000; // headroom under GitHub's 125,000 hard cap
const releaseBodyTemplate =
  "<% var notes = nextRelease.notes || ''; %>" +
  `<% if (notes.length <= ${RELEASE_BODY_MAX}) { %>` +
  "<%= notes %>" +
  "<% } else { %>" +
  `<%= notes.slice(0, ${RELEASE_BODY_MAX}) %>` +
  "\n\n---\n\n**Release notes truncated** — the full list exceeded GitHub's " +
  "125,000-character limit. Full changelog: " +
  `${repoBaseUrl}/blob/` +
  "<%= nextRelease.gitTag %>/frontend/public/CHANGELOG.md" +
  "<% } %>";

module.exports = {
  branches: ["main", { name: "beta", prerelease: true }],
  tagFormat: "v${version}",
  plugins: [
    ["@semantic-release/commit-analyzer", { releaseRules: depReleaseRules }],
    "@semantic-release/release-notes-generator",
    // changelogTitle keeps "# Changelog" pinned at the top and inserts each new
    // release directly beneath it, preserving the existing curated history below.
    [
      "@semantic-release/changelog",
      { changelogFile: "frontend/public/CHANGELOG.md", changelogTitle: "# Changelog" },
    ],
    [
      "@semantic-release/exec",
      {
        // Mirror the computed version into the two files the app/badge read.
        // printf (no trailing newline) matches the existing VERSION.txt format.
        prepareCmd:
          "printf 'v%s' \"${nextRelease.version}\" > VERSION.txt && jq --arg m \"v${nextRelease.version}\" '.message=$m' version.json > version.json.tmp && mv version.json.tmp version.json",
      },
    ],
    [
      "@semantic-release/git",
      {
        assets: ["VERSION.txt", "version.json", "frontend/public/CHANGELOG.md"],
        // Keep the release notes OUT of the commit message: the first release
        // rolls up ~1800 commits (~130 KB of notes), and inlining that via
        // `git commit -m` overruns the OS argument limit (E2BIG). The full notes
        // still land in CHANGELOG.md and the GitHub Release body.
        message: "chore(release): v${nextRelease.version} [skip ci]",
      },
    ],
    [
      "@semantic-release/github",
      { successComment: false, failComment: false, releaseBodyTemplate },
    ],
  ],
};
