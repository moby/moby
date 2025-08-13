#!/usr/bin/env bash
#
# This file is a wrapper around "go mod" and "go work" commands. It is used
# to update the vendor directory and to tidy go.mod in each module.

set -e

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ROOTDIR="${SCRIPTDIR}/.."

# Detect workspace mode. Release branches may not have a go.work, and
# use the api and client modules released from master / main.
in_workspace=0
if [ "$(go env GOWORK)" != "off" ]; then
	in_workspace=1
fi

tidy() (
	set -ex

	cd "$ROOTDIR"

	if [ "$in_workspace" -eq 1 ]; then
		( cd api    && go work sync )
		( cd client && go work sync )
	else
		( cd api    && go mod tidy )
		( cd client && go mod tidy )
	fi

	go mod tidy
)

vendor() (
	set -ex

	cd "${ROOTDIR}"
	GOWORK=off go mod tidy
	GOWORK=off go mod vendor
)

help() {
	printf "%s:\n" "$(basename "$0")"
	echo "  - tidy: run go mod tidy"
	echo "  - vendor: run go work vendor (or go mod vendor)"
	echo "  - all: run tidy && vendor"
	echo "  - help: show this help"
}

case "$1" in
	tidy) tidy ;;
	vendor) vendor ;;
	""|all) tidy && vendor ;;
	*) help ;;
esac
