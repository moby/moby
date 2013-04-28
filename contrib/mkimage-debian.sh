#!/bin/bash
set -e

# these should match the names found at http://www.debian.org/releases/
stableSuite='squeeze'
testingSuite='wheezy'
unstableSuite='sid'

# if suite is equal to this, it gets the "latest" tag
latestSuite="$testingSuite"

variant='minbase'
include='iproute,iputils-ping'

repo="$1"
suite="${2:-$latestSuite}"
mirror="${3:-}" # stick to the default debootstrap mirror if one is not provided

if [ ! "$repo" ]; then
	echo >&2 "usage: $0 repo [suite [mirror]]"
	echo >&2 "   ie: $0 tianon/debian squeeze"
	exit 1
fi

target="/tmp/docker-rootfs-debian-$suite-$$-$RANDOM"

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"
returnTo="$(pwd -P)"

set -x

# bootstrap
mkdir -p "$target"
sudo debootstrap --verbose --variant="$variant" --include="$include" "$suite" "$target" "$mirror"

cd "$target"

# create the image
img=$(sudo tar -c . | docker import -)

# tag suite
docker tag $img $repo $suite

if [ "$suite" = "$latestSuite" ]; then
	# tag latest
	docker tag $img $repo latest
fi

# test the image
docker run -i -t $repo:$suite echo success

# unstable's version numbers match testing (since it's mostly just a sandbox for testing), so it doesn't get a version number tag
if [ "$suite" != "$unstableSuite" -a "$suite" != 'unstable' ]; then
	# tag the specific version
	ver=$(docker run $repo:$suite cat /etc/debian_version)
	docker tag $img $repo $ver
fi

# cleanup
cd "$returnTo"
sudo rm -rf "$target"
