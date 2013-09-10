#!/bin/bash

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

# FIXME: break down bundles into sub-scripts
# FIXME: create all bundles in a single run for consistency.
#	If the bundles directory already exists, fail or erase it.

set -e

# We're a nice, sexy, little shell script, and people might try to run us;
# but really, they shouldn't. We want to be in a container!
RESOLVCONF=$(readlink --canonicalize /etc/resolv.conf)
grep -q "$RESOLVCONF" /proc/mounts || {
	echo "# I will only run within a container."
	echo "# Try this instead:"
	echo "docker build ."
	exit 1
}

# List of bundles to create when no argument is passed
DEFAULT_BUNDLES=(
	test
	binary
	ubuntu
)

VERSION=$(cat ./VERSION)
GITCOMMIT=$(git rev-parse --short HEAD)
if test -n "$(git status --porcelain)"
then
	GITCOMMIT="$GITCOMMIT-dirty"
fi

# Use these flags when compiling the tests and final binary
LDFLAGS="-X main.GITCOMMIT $GITCOMMIT -X main.VERSION $VERSION -d -w"


bundle() {
	bundlescript=$1
	bundle=$(basename $bundlescript)
	echo "---> Making bundle: $bundle"
	mkdir -p bundles/$VERSION/$bundle
	source $bundlescript $(pwd)/bundles/$VERSION/$bundle
}

main() {

	# We want this to fail if the bundles already exist.
	# This is to avoid mixing bundles from different versions of the code.
	mkdir -p bundles
	if [ -e "bundles/$VERSION" ]; then
		echo "bundles/$VERSION already exists. Removing."
		rm -fr bundles/$VERSION && mkdir bundles/$VERSION || exit 1
	fi
	SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
	if [ $# -lt 1 ]; then
		bundles=($DEFAULT_BUNDLES)
	else
		bundles=($@)
	fi
	for bundle in ${bundles[@]}; do
		bundle $SCRIPTDIR/make/$bundle
	done
	cat <<EOF
###############################################################################
Now run the resulting image, making sure that you set AWS_S3_BUCKET,
AWS_ACCESS_KEY, and AWS_SECRET_KEY environment variables:

docker run -e AWS_S3_BUCKET=get-staging.docker.io \\
              AWS_ACCESS_KEY=AKI1234... \\
              AWS_SECRET_KEY=sEs3mE... \\
              GPG_PASSPHRASE=sesame... \\
              image_id_or_name
###############################################################################
EOF
}

main "$@"
