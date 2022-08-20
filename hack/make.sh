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

if [ "$AUTO_GOPATH" ]; then
	rm -rf .gopath
	mkdir -p .gopath/src/"$(dirname "${DOCKER_PKG}")"
	ln -sf ../../../.. .gopath/src/"${DOCKER_PKG}"
	export GOPATH="${PWD}/.gopath"
fi

if [ ! "$GOPATH" ]; then
	echo >&2 'error: missing GOPATH; please see https://golang.org/doc/code.html#GOPATH'
	echo >&2 '  alternatively, set AUTO_GOPATH=1'
	exit 1
fi

# Adds $1_$2 to DOCKER_BUILDTAGS unless it already
# contains a word starting from $1_
add_buildtag() {
	[[ " $DOCKER_BUILDTAGS" == *" $1_"* ]] || DOCKER_BUILDTAGS+=" $1_$2"
}

if [ -z "$CGO_ENABLED" ]; then
	case "$(go env GOOS)/$(go env GOARCH)" in
		darwin/* | windows/amd64 | linux/amd64 | linux/arm64 | linux/arm | linux/s390x | linux/ppc64le | linux/riscv*)
			export CGO_ENABLED=1
			;;
		*)
			export CGO_ENABLED=0
			;;
	esac
fi

if [ "$CGO_ENABLED" = "1" ] && [ "$DOCKER_LINKMODE" = "static" ] && [ "$(go env GOOS)" = "linux" ]; then
	DOCKER_LDFLAGS+=" -extldflags -static"
fi

if [ "$(uname -s)" = 'FreeBSD' ]; then
	# Tell cgo the compiler is Clang, not GCC
	# https://code.google.com/p/go/source/browse/src/cmd/cgo/gcc.go?spec=svne77e74371f2340ee08622ce602e9f7b15f29d8d3&r=e6794866ebeba2bf8818b9261b54e2eef1c9e588#752
	export CC=clang

	# "-extld clang" is a workaround for
	# https://code.google.com/p/go/issues/detail?id=6845
	DOCKER_LDFLAGS+=" -extld clang"
fi

if [ "$CGO_ENABLED" = "1" ] && [ "$DOCKER_LINKMODE" = "static" ]; then
	DOCKER_BUILDTAGS+=" netgo osusergo static_build"
fi

if [ "$CGO_ENABLED" = "1" ] && [ "$(go env GOOS)" != "windows" ] && [ "$DOCKER_LINKMODE" != "static" ]; then
	# pkcs11 cannot be compiled statically if CGO is enabled (and glibc is used)
	DOCKER_BUILDTAGS+=" pkcs11"
fi

if ${PKG_CONFIG} 'libsystemd' 2> /dev/null; then
	DOCKER_BUILDTAGS+=" journald"
fi

if [ "$DOCKER_LINKMODE" != "static" ]; then
	# test whether "libdevmapper.h" is new enough to support deferred remove
	# functionality. We favour libdm_dlsym_deferred_remove over
	# libdm_no_deferred_remove in dynamic cases because the binary could be shipped
	# with a newer libdevmapper than the one it was built with.
	if command -v gcc &> /dev/null && ! (echo -e '#include <libdevmapper.h>\nint main() { dm_task_deferred_remove(NULL); }' | gcc -xc - -o /dev/null $(${PKG_CONFIG} --libs devmapper 2> /dev/null) &> /dev/null); then
		add_buildtag libdm dlsym_deferred_remove
	fi
fi

export DOCKER_LDFLAGS
export DOCKER_BUILDFLAGS=(-tags "${DOCKER_BUILDTAGS}" -installsuffix netgo)
# see https://github.com/golang/go/issues/9369#issuecomment-69864440 for why -installsuffix is necessary here

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
