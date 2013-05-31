#!/bin/bash
set -e

# these should match the names found at http://www.debian.org/releases/
stableSuite='wheezy'
testingSuite='jessie'
unstableSuite='sid'

variant='minbase'
include='iproute,iputils-ping'

repo="$1"
suite="${2:-$stableSuite}"
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

# test the image
docker run -i -t $repo:$suite echo success

if [ "$suite" = "$stableSuite" -o "$suite" = 'stable' ]; then
	# tag latest
	docker tag $img $repo latest
	
	# tag the specific debian release version
	ver=$(docker run $repo:$suite cat /etc/debian_version)
	docker tag $img $repo $ver
fi

# cleanup
cd "$returnTo"
sudo rm -rf "$target"
