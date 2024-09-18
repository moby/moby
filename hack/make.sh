#!/usr/bin/env bash
set -e

# This script builds various binary artifacts from a checkout of the docker
# source code.
#
# Requirements:
# - The current directory should be a checkout of the docker source code
#   (https://github.com/docker/docker). Whatever version is checked out
#   will be built.
# - The VERSION file, at the root of the repository, should exist, and
#   will be used as Docker binary version and package version.
# - The hash of the git commit will also be included in the Docker binary,
#   with the suffix -unsupported if the repository isn't clean.
# - The script is intended to be run inside the docker container specified
#   in the Dockerfile at the root of the source. In other words:
#   DO NOT CALL THIS SCRIPT DIRECTLY.
# - The right way to call this script is to invoke "make" from
#   your checkout of the Docker repository.
#   the Makefile will do a "docker build -t docker ." and then
#   "docker run hack/make.sh" in the resulting image.
#

set -o pipefail

export DOCKER_PKG='github.com/docker/docker'
export SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export MAKEDIR="$SCRIPTDIR/make"
export PKG_CONFIG=${PKG_CONFIG:-pkg-config}

echo

# List of bundles to create when no argument is passed
DEFAULT_BUNDLES=(
	binary-daemon
	dynbinary
	test-integration
	test-docker-py
)

VERSION=${VERSION:-dev}
case "$VERSION" in
	refs/tags/v*) VERSION=${VERSION#refs/tags/v} ;;
	refs/tags/*) VERSION=${VERSION#refs/tags/} ;;
	refs/heads/*) VERSION=$(echo "${VERSION#refs/heads/}" | sed -r 's#/+#-#g') ;;
	refs/pull/*) VERSION=pr-$(echo "$VERSION" | grep -o '[0-9]\+') ;;
esac

! BUILDTIME=$(date -u -d "@${SOURCE_DATE_EPOCH:-$(date +%s)}" --rfc-3339 ns 2> /dev/null | sed -e 's/ /T/')
if [ "$DOCKER_GITCOMMIT" ]; then
	GITCOMMIT="$DOCKER_GITCOMMIT"
elif command -v git &> /dev/null && [ -e .git ] && git rev-parse &> /dev/null; then
	GITCOMMIT=$(git rev-parse HEAD)
	if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
		GITCOMMIT="$GITCOMMIT-unsupported"
		echo "#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
		echo "# GITCOMMIT = $GITCOMMIT"
		echo "# The version you are building is listed as unsupported because"
		echo "# there are some files in the git repository that are in an uncommitted state."
		echo "# Commit these changes, or add to .gitignore to remove the -unsupported from the version."
		echo "# Here is the current list:"
		git status --porcelain --untracked-files=no
		echo "#~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
	fi
else
	echo >&2 'error: .git directory missing and DOCKER_GITCOMMIT not specified'
	echo >&2 '  Please either build with the .git directory accessible, or specify the'
	echo >&2 '  exact commit hash you are building using DOCKER_GITCOMMIT for future'
	echo >&2 '  accountability in diagnosing build issues.  Thanks!'
	exit 1
fi

if [ "$AUTO_GOPATH" ]; then
	rm -rf .gopath
	mkdir -p .gopath/src/"$(dirname "${DOCKER_PKG}")"
	ln -sf ../../../.. .gopath/src/"${DOCKER_PKG}"
	export GOPATH="${PWD}/.gopath"
fi

if [ ! "$GOPATH" ]; then
	echo >&2 'error: missing GOPATH; please see https://pkg.go.dev/cmd/go#hdr-GOPATH_environment_variable'
	echo >&2 '  alternatively, set AUTO_GOPATH=1'
	exit 1
fi

if [ -z "${EXCLUDE_AUTO_BUILDTAG_JOURNALD:-}" ] && ${PKG_CONFIG} 'libsystemd' 2> /dev/null; then
	DOCKER_BUILDTAGS+=" journald"
fi

# Use these flags when compiling the tests and final binary

if [ -z "$DOCKER_DEBUG" ]; then
	LDFLAGS='-w'
fi

BUILDFLAGS=(${BUILDFLAGS} -tags "netgo osusergo static_build $DOCKER_BUILDTAGS")
LDFLAGS_STATIC="-extldflags -static"

if [ "$(uname -s)" = 'FreeBSD' ]; then
	# Tell cgo the compiler is Clang, not GCC
	# https://code.google.com/p/go/source/browse/src/cmd/cgo/gcc.go?spec=svne77e74371f2340ee08622ce602e9f7b15f29d8d3&r=e6794866ebeba2bf8818b9261b54e2eef1c9e588#752
	export CC=clang

	# "-extld clang" is a workaround for
	# https://code.google.com/p/go/issues/detail?id=6845
	LDFLAGS="$LDFLAGS -extld clang"
fi

bundle() {
	local bundle="$1"
	shift
	echo "---> Making bundle: $(basename "$bundle") (in $DEST)"
	source "$SCRIPTDIR/make/$bundle" "$@"
}

main() {
	bundle_dir="bundles"
	if [ -n "${PREFIX}" ]; then
		bundle_dir="${PREFIX}/${bundle_dir}"
	fi

	if [ -z "${KEEPBUNDLE-}" ]; then
		echo "Removing ${bundle_dir}/"
		rm -rf "${bundle_dir}"/*
		echo
	fi
	mkdir -p "${bundle_dir}"

	if [ $# -lt 1 ]; then
		bundles=(${DEFAULT_BUNDLES[@]})
	else
		bundles=($@)
	fi
	for bundle in ${bundles[@]}; do
		export DEST="${bundle_dir}/$(basename "$bundle")"
		# Cygdrive paths don't play well with go build -o.
		if [[ "$(uname -s)" == CYGWIN* ]]; then
			export DEST="$(cygpath -mw "$DEST")"
		fi
		mkdir -p "$DEST"
		ABS_DEST="$(cd "$DEST" && pwd -P)"
		bundle "$bundle"
		echo
	done
}

main "$@"
