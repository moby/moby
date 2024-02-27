#!/usr/bin/env bash
#
# This script is used to coerce certain commands which rely on the presence of
# a go.mod into working with our repository. It works by creating a fake
# go.mod, running a specified command (passed via arguments), and removing it
# when the command is finished. This script should be dropped when this
# repository is a proper Go module with a permanent go.mod.

set -e

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOTDIR="$(cd "${SCRIPTDIR}/.." && pwd)"

if test -e "${ROOTDIR}/go.mod"; then
	{
		scriptname=$(basename "$0")
		cat >&2 <<- EOF
			$scriptname: WARN: go.mod exists in the repository root!
			$scriptname: WARN: Using your go.mod instead of our generated version -- this may misbehave!
		EOF
	} >&2
else
	set -x

	tee "${ROOTDIR}/go.mod" >&2 <<- EOF
		module github.com/docker/docker

		go 1.21
	EOF
	trap 'rm -f "${ROOTDIR}/go.mod"' EXIT
fi

GO111MODULE=on GOTOOLCHAIN=local "$@"
