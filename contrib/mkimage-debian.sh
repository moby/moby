#!/bin/bash
set -e

# these should match the names found at http://www.debian.org/releases/
stableSuite='wheezy'
testingSuite='jessie'
unstableSuite='sid'

variant='minbase'
include='iproute,iputils-ping'

repo="$1"
suite="$2"
mirror="${3:-}" # stick to the default debootstrap mirror if one is not provided

if [ ! "$repo" ] || [ ! "$suite" ]; then
	echo >&2 "usage: $0 repo suite [mirror]"
	echo >&2
	echo >&2 "   ie: $0 tianon/debian squeeze"
	echo >&2 "       $0 tianon/debian squeeze http://ftp.uk.debian.org/debian/"
	echo >&2
	echo >&2 "   ie: $0 tianon/ubuntu precise"
	echo >&2 "       $0 tianon/ubuntu precise http://mirrors.melbourne.co.uk/ubuntu/"
	echo >&2
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
#  policy-rc.d (for most scripts)
echo $'#!/bin/sh\nexit 101' | sudo tee usr/sbin/policy-rc.d > /dev/null
sudo chmod +x usr/sbin/policy-rc.d
#  initctl (for some pesky upstart scripts)
sudo chroot . dpkg-divert --local --rename --add /sbin/initctl
sudo ln -sf /bin/true sbin/initctl
# see https://github.com/dotcloud/docker/issues/446#issuecomment-16953173

# shrink the image, since apt makes us fat (wheezy: ~157.5MB vs ~120MB)
sudo chroot . apt-get clean

# while we're at it, apt is unnecessarily slow inside containers
#  this forces dpkg not to call sync() after package extraction and speeds up install
#    the benefit is huge on spinning disks, and the penalty is nonexistent on SSD or decent server virtualization
echo 'force-unsafe-io' | sudo tee etc/dpkg/dpkg.cfg.d/02apt-speedup > /dev/null
#  we want to effectively run "apt-get clean" after every install to keep images small
echo 'DPkg::Post-Invoke {"/bin/rm -f /var/cache/apt/archives/*.deb || true";};' | sudo tee etc/apt/apt.conf.d/no-cache > /dev/null

# helpful undo lines for each the above tweaks (for lack of a better home to keep track of them):
#  rm /usr/sbin/policy-rc.d
#  rm /sbin/initctl; dpkg-divert --rename --remove /sbin/initctl
#  rm /etc/dpkg/dpkg.cfg.d/02apt-speedup
#  rm /etc/apt/apt.conf.d/no-cache

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
