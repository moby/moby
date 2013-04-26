#!/bin/bash
set -e

latestSuite='wheezy'

repo="$1"
suite="${2:-$latestSuite}"
mirror="${3:-http://ftp.us.debian.org/debian}"

if [ ! "$repo" ]; then
	echo >&2 "usage: $0 repo [suite [mirror]]"
	echo >&2 "   ie: $0 tianon/debian squeeze"
	exit 1
fi

target="/tmp/docker-rootfs-$$-$RANDOM-debian-$suite"

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"
returnTo="$(pwd -P)"

set -x

# bootstrap
mkdir -p "$target"
sudo debootstrap --verbose --variant=minbase --include=iproute,iputils-ping "$suite" "$target" "$mirror"

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

# cleanup
cd "$returnTo"
sudo rm -rf "$target"
