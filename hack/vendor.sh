#!/usr/bin/env bash
#
# This file is just a wrapper around the 'go mod vendor' tool.
# For updating dependencies you should change `vendor.mod` file in root of the
# project.

set -e

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

tidy() (
		set -x
		"${SCRIPTDIR}"/with-go-mod.sh go mod tidy -modfile vendor.mod -compat 1.18
)

vendor() (
		set -x
		"${SCRIPTDIR}"/with-go-mod.sh go mod vendor -modfile vendor.mod
)

help() {
	printf "%s:\n" "$(basename "$0")"
	echo "  - tidy: run go mod tidy"
	echo "  - vendor: run go mod vendor"
	echo "  - all: run tidy && vendor"
	echo "  - help: show this help"
}

case "$1" in
	tidy) tidy ;;
	vendor) vendor ;;
	""|all) tidy && vendor ;;
	*) help ;;
esac
