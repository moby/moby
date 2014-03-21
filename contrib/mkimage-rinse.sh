#!/usr/bin/env bash
#
# Create a base CentOS Docker image.

# This script is useful on systems with rinse available (e.g.,
# building a CentOS image on Debian).  See contrib/mkimage-yum.sh for
# a way to build CentOS images on systems with yum installed.

set -e

function usage() {
    self="$(basename $0)"
    cat << EOF
usage: $self --repo repo --distro distro [--mirror mirror]
   [--arch (amd64|i386)] [--add-pkg-list <file>]
   [--before-post-install <script>]
   [--after-post-install <script>]
   [--post-install <script>]

   ie: $self --repo username/centos --distro centos-5
       $self --repo username/centos --distro centos-6

   ie: $self --repo username/slc --distro slc-5
       $self --repo username/slc --distro slc-6

   ie: $self --repo username/centos --distro centos-5 --mirror  http://vault.centos.org/5.8/os/x86_64/CentOS/
       $self --repo username/centos --distro centos-6 --mirror  http://vault.centos.org/6.3/os/x86_64/Packages/

See /etc/rinse for supported values of "distro" and for examples of
  expected values of "mirror".

This script was tested with rinse 2.0.1. You can find it
at http://www.steve.org.uk/Software/rinse/ and also in Debian at
http://packages.debian.org/wheezy/rinse -- as always, YMMV.

EOF
exit 1
}

repo=
distro=
mirror=
arch=amd64
add_pkg_list=
after_post_install=
before_post_install=
post_install=

while test $# -gt 0; do
    case "$1" in
        -h|--help)
            usage
            ;;
        -r|--repo)
            repo=$2
            shift 2
            ;;
        -d|--distro)
            distro=$2
            shift 2
            ;;
        -m|--mirror)
            mirror=$2
            shift 2
            ;;
        -a|--arch)
            arch=$2
            shift 2
            ;;
        --add-pkg-list)
            add_pkg_list=$2
            shift 2
            ;;
        --after-post-install)
            after_post_install=$2
            shift 2
            ;;
        --before-post-install)
            before_post_install=$2
            shift 2
            ;;
        --post-install)
            post_install=$2
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

if [ -z "$repo" ] || [ -z "$distro" ]; then
    usage
fi

if [ "$arch" != "amd64" ] && [ "$arch" != "i386" ]; then
    echo "Arch must be either amd64 or i386"
    usage
fi

target="/tmp/docker-rootfs-rinse-$distro-$$-$RANDOM"

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"
returnTo="$(pwd -P)"

rinseArgs=( --arch "$arch" --distribution "$distro" --directory "$target" )
if [ "$mirror" ]; then
	rinseArgs+=( --mirror "$mirror" )
fi

if [ "$add_pkg_list" ]; then
	rinseArgs+=( --add-pkg-list "$add_pkg_list" )
fi

if [ "$after_post_install" ]; then
	rinseArgs+=( --after-post-install "$after_post_install" )
fi

if [ "$before_post_install" ]; then
	rinseArgs+=( --before-post-install "$before_post_install" )
fi

if [ "$post_install" ]; then
	rinseArgs+=( --post-install "$post_install" )
fi

set -x

mkdir -p "$target"

sudo rinse "${rinseArgs[@]}"

cd "$target"

# rinse fails a little at setting up /dev, so we'll just wipe it out and create our own
sudo rm -rf dev
sudo mkdir -m 755 dev
(
	cd dev
	sudo ln -sf /proc/self/fd ./
	sudo mkdir -m 755 pts
	sudo mkdir -m 1777 shm
	sudo mknod -m 600 console c 5 1
	sudo mknod -m 600 initctl p
	sudo mknod -m 666 full c 1 7
	sudo mknod -m 666 null c 1 3
	sudo mknod -m 666 ptmx c 5 2
	sudo mknod -m 666 random c 1 8
	sudo mknod -m 666 tty c 5 0
	sudo mknod -m 666 tty0 c 4 0
	sudo mknod -m 666 urandom c 1 9
	sudo mknod -m 666 zero c 1 5
)

# effectively: febootstrap-minimize --keep-zoneinfo --keep-rpmdb --keep-services "$target"
#  locales
sudo rm -rf usr/{{lib,share}/locale,{lib,lib64}/gconv,bin/localedef,sbin/build-locale-archive}
#  docs
sudo rm -rf usr/share/{man,doc,info,gnome/help}
#  cracklib
sudo rm -rf usr/share/cracklib
#  i18n
sudo rm -rf usr/share/i18n
#  yum cache
sudo rm -rf var/cache/yum
sudo mkdir -p --mode=0755 var/cache/yum
#  sln
sudo rm -rf sbin/sln
#  ldconfig
#sudo rm -rf sbin/ldconfig
sudo rm -rf etc/ld.so.cache var/cache/ldconfig
sudo mkdir -p --mode=0755 var/cache/ldconfig

# allow networking init scripts inside the container to work without extra steps
echo 'NETWORKING=yes' | sudo tee etc/sysconfig/network > /dev/null

# to restore locales later:
#  yum reinstall glibc-common

version=
if [ -r etc/redhat-release ]; then
	version="$(sed -E 's/^[^0-9.]*([0-9.]+).*$/\1/' etc/redhat-release)"
elif [ -r etc/SuSE-release ]; then
	version="$(awk '/^VERSION/ { print $3 }' etc/SuSE-release)"
fi

if [ -z "$version" ]; then
	echo >&2 "warning: cannot autodetect OS version, using $distro as tag"
	sleep 20
	version="$distro"
fi

sudo tar --numeric-owner -c . | docker import - $repo:$version

docker run -i -t --rm=true $repo:$version echo success

cd "$returnTo"
sudo rm -rf "$target"
