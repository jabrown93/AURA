#!/bin/sh
# Runs go vet and every backend test. Extra arguments are passed to go test, e.g. ./run_tests.sh -race or ./run_tests.sh -run Backup
set -e
cd "$(dirname "$0")"

# Tests never use the real config folder
CONFIG_PATH="$(mktemp -d)"
export CONFIG_PATH
unset DATABASE_PATH
trap 'rm -rf "$CONFIG_PATH"' EXIT

go vet ./...
go test "$@" ./...

# Every package should have tests. api-docs is generated, so it has none
untested=$(go list -f '{{if and (not .TestGoFiles) (not .XTestGoFiles)}}{{.ImportPath}}{{end}}' ./... | grep -v '^aura/api-docs$' || true)
if [ -n "$untested" ]; then
	echo
	echo "Packages without tests:"
	echo "$untested" | sed 's/^/  /'
fi
