#!/usr/bin/env bash
#
# This file is just a wrapper around the 'go mod vendor' tool.
# For updating dependencies you should change `go.mod` file in root of the
# project.

set -e

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

tidy() (
		set -x
		go mod tidy
)

vendor() (
		set -x
		go mod vendor
)

replace() (
	set -x
	go mod edit -replace=github.com/moby/moby/api=./api -replace=github.com/moby/moby/client=./client
	go mod edit -modfile client/go.mod -replace=github.com/moby/moby/api=../api
)

dropreplace() (
	set -x
	go mod edit -dropreplace=github.com/moby/moby/api -dropreplace=github.com/moby/moby/client
	go mod edit -modfile client/go.mod -dropreplace=github.com/moby/moby/api
)

help() {
	printf "%s:\n" "$(basename "$0")"
	echo "  - tidy: run go mod tidy"
	echo "  - vendor: run go mod vendor"
	echo "  - replace: run go mod edit replace for local modules"
	echo "  - dropreplace: run go mod edit dropreplace for local modules"
	echo "  - all: run tidy && vendor"
	echo "  - help: show this help"
}

case "$1" in
	tidy) tidy ;;
	vendor) vendor ;;
	replace) replace ;;
	dropreplace) dropreplace ;;
	""|all) tidy && vendor ;;
	*) help ;;
esac
