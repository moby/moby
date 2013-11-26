#!/bin/bash
set -e

# This script builds various binary artifacts from a checkout of the docker
# source code.
#
# Requirements:
# - The current directory should be a checkout of the docker source code
#   (http://github.com/dotcloud/docker). Whatever version is checked out
#   will be built.
# - The VERSION file, at the root of the repository, should exist, and
#   will be used as Docker binary version and package version.
# - The hash of the git commit will also be included in the Docker binary,
#   with the suffix -dirty if the repository isn't clean.
# - The script is intented to be run inside the docker container specified
#   in the Dockerfile at the root of the source. In other words:
#   DO NOT CALL THIS SCRIPT DIRECTLY.
# - The right way to call this script is to invoke "docker build ." from
#   your checkout of the Docker repository, and then
#   "docker run hack/make.sh" in the resulting container image.
#

set -o pipefail

# We're a nice, sexy, little shell script, and people might try to run us;
# but really, they shouldn't. We want to be in a container!
RESOLVCONF=$(readlink --canonicalize /etc/resolv.conf)
grep -q "$RESOLVCONF" /proc/mounts || {
	echo >&2 "# WARNING! I don't seem to be running in a docker container."
	echo >&2 "# The result of this command might be an incorrect build, and will not be officially supported."
	echo >&2 "# Try this: 'docker build -t docker . && docker run docker ./hack/make.sh'"
}

# List of bundles to create when no argument is passed
DEFAULT_BUNDLES=(
	binary
	test
	dynbinary
	dyntest
	tgz
	ubuntu
)

VERSION=$(cat ./VERSION)
if [ -d .git ] && command -v git &> /dev/null; then
	GITCOMMIT=$(git rev-parse --short HEAD)
	if [ -n "$(git status --porcelain)" ]; then
		GITCOMMIT="$GITCOMMIT-dirty"
	fi
elif [ "$DOCKER_GITCOMMIT" ]; then
	GITCOMMIT="$DOCKER_GITCOMMIT"
else
	echo >&2 'error: .git directory missing and DOCKER_GITCOMMIT not specified'
	echo >&2 '  Please either build with the .git directory accessible, or specify the'
	echo >&2 '  exact (--short) commit hash you are building using DOCKER_GITCOMMIT for'
	echo >&2 '  future accountability in diagnosing build issues.  Thanks!'
	exit 1
fi

# Use these flags when compiling the tests and final binary
LDFLAGS='-X main.GITCOMMIT "'$GITCOMMIT'" -X main.VERSION "'$VERSION'" -w'
LDFLAGS_STATIC='-X github.com/dotcloud/docker/utils.IAMSTATIC true -linkmode external -extldflags "-lpthread -static -Wl,--unresolved-symbols=ignore-in-object-files"'
BUILDFLAGS='-tags netgo'

bundle() {
	bundlescript=$1
	bundle=$(basename $bundlescript)
	echo "---> Making bundle: $bundle (in bundles/$VERSION/$bundle)"
	mkdir -p bundles/$VERSION/$bundle
	source $bundlescript $(pwd)/bundles/$VERSION/$bundle
}

main() {
	# We want this to fail if the bundles already exist and cannot be removed.
	# This is to avoid mixing bundles from different versions of the code.
	mkdir -p bundles
	if [ -e "bundles/$VERSION" ]; then
		echo "bundles/$VERSION already exists. Removing."
		rm -fr bundles/$VERSION && mkdir bundles/$VERSION || exit 1
		echo
	fi
	SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
	if [ $# -lt 1 ]; then
		bundles=(${DEFAULT_BUNDLES[@]})
	else
		bundles=($@)
	fi
	for bundle in ${bundles[@]}; do
		bundle $SCRIPTDIR/make/$bundle
		echo
	done
}

main "$@"
