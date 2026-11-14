#!/bin/sh
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT HUP INT TERM
mkdir -p "$fixture/scripts" "$fixture/backend/database" "$fixture/backend/models" "$fixture/backend/config"
cp "$root/scripts/check-postgres-compat.sh" "$fixture/scripts/"
cd "$fixture"
printf 'package database\n' > backend/database/sqlite_init.go
sh scripts/check-postgres-compat.sh --record
sh scripts/check-postgres-compat.sh
printf 'unrelated documentation\n' > README.md
printf 'package database\n' > backend/database/fork_postgres.go
sh scripts/check-postgres-compat.sh
printf 'type NewColumn string\n' >> backend/database/sqlite_init.go
if sh scripts/check-postgres-compat.sh; then echo 'Missed changed storage file' >&2; exit 1; fi
sh scripts/check-postgres-compat.sh --record
printf 'package database\n' > backend/database/sqlite_new.go
if sh scripts/check-postgres-compat.sh; then echo 'Missed added storage file' >&2; exit 1; fi
printf 'PostgreSQL compatibility guard tests passed.\n'
