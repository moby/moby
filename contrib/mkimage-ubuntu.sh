#!/bin/bash
set -e

variant='minbase'
include='iproute,iputils-ping,net-tools'
default_suite='raring'
default_mirror='http://archive.ubuntu.com/ubuntu'
default_arch='amd64'

docker_repo="${1}"
suite="${2:-$default_suite}"
arch="${3:-$default_arch}"
mirror="${4:-$default_mirror}"

if [ -z "$docker_repo" -o "$docker_repo" == "-h" -o "$docker_repo" == "--h" ]; then
  echo >&2 "usage: $0 docker_repo [suite [arch [mirror]]]"
  echo >&2 "  default [suite] is ${default_suite}, ex: precise, quantal, saucy"
  echo >&2 "  default [arch] is ${default_arch}, ex: i386"
  echo >&2 "  default [mirror] is ${default_mirror}"
  echo >&2 "  ex.: $0 joeuser/ubuntu precise amd64 http://ca.archive.ubuntu.com/ubuntu"
  exit 1
fi

suite_arch="${suite}-${arch}"
target="/tmp/docker-rootfs-${suite_arch}-$$-$RANDOM"

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"
returnTo="$(pwd -P)"

set -x

# bootstrap
mkdir -p "$target"
sudo debootstrap --verbose --variant="$variant" --arch="$arch" --include="$include" "$suite" "$target" "$mirror"

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

# create the image (and tag $docker_repo:$suite)
sudo tar -c . | docker import - $docker_repo $suite_arch

# test the image
docker run -i -t $docker_repo:$suite_arch echo SUCCESS

# cleanup
cd "$returnTo"
sudo rm -rf "$target"

set +x
echo ""
echo "add more tags using: docker tag $docker_repo:$suite_arch $docker_repo [tag]"