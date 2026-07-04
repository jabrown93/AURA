// semantic-release configuration.
//
// Versioning is automated from Conventional Commits:
//   * push to `main` -> stable release (feat -> minor, fix/perf -> patch, ! -> major)
//   * push to `beta` -> prerelease (vX.Y.Z-beta.N)
//
// Dependency bumps (chore(deps)/build(deps)) intentionally do NOT cut a release
// on ordinary pushes. The weekly scheduled workflow run sets RELEASE_DEPS=true,
// which promotes accumulated dependency commits to a single patch release.
//
// This file is CommonJS (there is no root package.json); semantic-release loads
// it via cosmiconfig. `${...}` placeholders are expanded by semantic-release, not
// by JS — keep them inside double-quoted strings so JS does not interpolate them.

const releaseDeps = process.env.RELEASE_DEPS === "true";

const depReleaseRules = releaseDeps
  ? [
      { type: "chore", scope: "deps", release: "patch" },
      { type: "build", scope: "deps", release: "patch" },
    ]
  : [];

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
    ["@semantic-release/github", { successComment: false, failComment: false }],
  ],
};
