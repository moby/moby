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

	# Disable workspace when tidying the api and client modules to prevent
	# common dependencies between other modules in the workspace from affecting
	# the other modules. This allows us to stick to MVS for the api and client
	# modules, while still updating the main ("v2") module's dependencies.
	( cd api    && GOWORK=off go mod tidy )
	( cd client && GOWORK=off go mod tidy )

	go mod tidy
)

vendor() (
	set -ex

	cd "$ROOTDIR"

	if [ "$in_workspace" -eq 1 ]; then
		go work vendor
	else
		go mod vendor
	fi
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
