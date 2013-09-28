#!/bin/sh
set -e
#
# This script is meant for quick & easy install via:
#   'curl -sL https://get.docker.io/ | sh'
# or:
#   'wget -qO- https://get.docker.io/ | sh'
#
#
# Docker Maintainers:
#   To update this script on https://get.docker.io,
#   use hack/release.sh during a normal release,
#   or the following one-liner for script hotfixes:
#     s3cmd put --acl-public -P hack/install.sh s3://get.docker.io/index
#

url='https://get.docker.io/'

command_exists() {
	command -v "$@" > /dev/null 2>&1
}

case "$(uname -m)" in
	*64)
		;;
	*)
		echo >&2 'Error: you are not using a 64bit platform.'
		echo >&2 'Docker currently only supports 64bit platforms.'
		exit 1
		;;
esac

if command_exists docker || command_exists lxc-docker; then
	echo >&2 'Warning: "docker" or "lxc-docker" command appears to already exist.'
	echo >&2 'Please ensure that you do not already have docker installed.'
	echo >&2 'You may press Ctrl+C now to abort this process and rectify this situation.'
	( set -x; sleep 20 )
fi

sh_c='sh -c'
if [ "$(whoami 2>/dev/null || true)" != 'root' ]; then
	if command_exists sudo; then
		sh_c='sudo sh -c'
	elif command_exists su; then
		sh_c='su -c'
	else
		echo >&2 'Error: this installer needs the ability to run commands as root.'
		echo >&2 'We are unable to find either "sudo" or "su" available to make this happen.'
		exit 1
	fi
fi

curl=''
if command_exists curl; then
	curl='curl -sL'
elif command_exists wget; then
	curl='wget -qO-'
elif command_exists busybox && busybox --list-modules | grep -q wget; then
	curl='busybox wget -qO-'
fi

# perform some very rudimentary platform detection
lsb_dist=''
if command_exists lsb_release; then
	lsb_dist="$(lsb_release -si)"
fi
if [ -z "$lsb_dist" ] && [ -r /etc/lsb-release ]; then
	lsb_dist="$(. /etc/lsb-release && echo "$DISTRIB_ID")"
fi
if [ -z "$lsb_dist" ] && [ -r /etc/debian_version ]; then
	lsb_dist='Debian'
fi

case "$lsb_dist" in
	Ubuntu|Debian)
		export DEBIAN_FRONTEND=noninteractive
		
		# TODO remove this comment/section once device-mapper lands
		echo 'Warning: Docker currently requires AUFS support in the kernel.'
		echo 'Please ensure that your kernel includes such support.'
		( set -x; sleep 10 )
		
		if [ ! -e /usr/lib/apt/methods/https ]; then
			( set -x; $sh_c 'sleep 3; apt-get update; apt-get install -y -q apt-transport-https' )
		fi
		if [ -z "$curl" ]; then
			( set -x; $sh_c 'sleep 3; apt-get update; apt-get install -y -q curl' )
			curl='curl -sL'
		fi
		(
			set -x
			$sh_c "$curl ${url}gpg | apt-key add -"
			$sh_c "echo deb ${url}ubuntu docker main > /etc/apt/sources.list.d/docker.list"
			$sh_c 'sleep 3; apt-get update; apt-get install -y -q lxc-docker'
		)
		if command_exists docker && [ -e /var/run/docker.sock ]; then
			(
				set -x
				$sh_c 'docker run busybox echo "Docker has been successfully installed!"'
			)
		fi
		exit 0
		;;
		
	Gentoo)
		if [ "$url" = "https://test.docker.io/" ]; then
			echo >&2
			echo >&2 '  You appear to be trying to install the latest nightly build in Gentoo.'
			echo >&2 '  The portage tree should contain the latest stable release of Docker, but'
			echo >&2 '  if you want something more recent, you can always use the live ebuild'
			echo >&2 '  provided in the "docker" overlay available via layman.  For more'
			echo >&2 '  instructions, please see the following URL:'
			echo >&2 '    https://github.com/tianon/docker-overlay#using-this-overlay'
			echo >&2 '  After adding the "docker" overlay, you should be able to:'
			echo >&2 '    emerge -av =app-emulation/docker-9999'
			echo >&2
			exit 1
		fi
		
		(
			set -x
			$sh_c 'sleep 3; emerge app-emulation/docker'
		)
		exit 0
		;;
esac

echo >&2
echo >&2 '  Either your platform is not easily detectable, is not supported by this'
echo >&2 '  installer script (yet - PRs welcome!), or does not yet have a package for'
echo >&2 '  Docker.  Please visit the following URL for more detailed installation'
echo >&2 '  instructions:'
echo >&2
echo >&2 '    http://docs.docker.io/en/latest/installation/'
echo >&2
exit 1
