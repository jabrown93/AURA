#!/bin/sh
set -eu
cd "$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
if [ "$#" -gt 1 ] || { [ "$#" -eq 1 ] && [ "$1" != '--record' ]; }; then
  echo 'usage: scripts/check-postgres-compat.sh [--record]' >&2
  exit 2
fi
if command -v sha256sum >/dev/null 2>&1; then
  hash() { sha256sum "$@"; }
else
  hash() { shasum -a 256 "$@"; }
fi
manifest=$(mktemp)
trap 'rm -f "$manifest"' EXIT HUP INT TERM
# Include new upstream files, but not fork implementations or tests that cannot change runtime storage contracts.
find backend/database backend/models backend/config -type f -name '*.go' ! -name 'fork_*' ! -name '*_test.go' -print |
  LC_ALL=C sort | while IFS= read -r file; do hash "$file"; done > "$manifest"
digest=$(hash "$manifest" | awk '{print $1}')
if [ "${1:-}" = '--record' ]; then
  printf '%s\n' "$digest" > .postgres-storage-contract
  echo 'Recorded PostgreSQL storage review baseline. Commit baseline with reviewed changes.'
elif [ ! -f .postgres-storage-contract ] || [ "$(tr -d '\r\n' < .postgres-storage-contract)" != "$digest" ]; then
  echo 'Storage contract changed. Review PostgreSQL SQL, schema, import, and shared tests against SQLite changes.' >&2
  echo 'After review and passing both-engine tests: scripts/check-postgres-compat.sh --record' >&2
  exit 1
else
  echo 'PostgreSQL storage review baseline matches.'
fi
