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

# prevent init scripts from running during install/update
echo $'#!/bin/sh\nexit 101' | sudo tee usr/sbin/policy-rc.d > /dev/null
sudo chmod +x usr/sbin/policy-rc.d
# see https://github.com/dotcloud/docker/issues/446#issuecomment-16953173

# shrink the image, since apt makes us fat (wheezy: ~157.5MB vs ~120MB)
sudo chroot . apt-get clean

# while we're at it, apt is unnecessarily slow inside containers
#  this forces dpkg not to call sync() after package extraction and speeds up install
echo 'force-unsafe-io' | sudo tee etc/dpkg/dpkg.cfg.d/02apt-speedup > /dev/null
#  we don't need an apt cache in a container
echo 'Acquire::http {No-Cache=True;};' | sudo tee etc/apt/apt.conf.d/no-cache > /dev/null

# create the image (and tag $repo:$suite)
sudo tar -c . | docker import - $repo $suite

# test the image
docker run -i -t $repo:$suite echo success

if [ "$suite" = "$stableSuite" -o "$suite" = 'stable' ]; then
	# tag latest
	docker tag $repo:$suite $repo latest
	
	# tag the specific debian release version
	ver=$(docker run $repo:$suite cat /etc/debian_version)
	docker tag $repo:$suite $repo $ver
fi

# cleanup
cd "$returnTo"
sudo rm -rf "$target"
