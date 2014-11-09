#!/bin/sh
set -e
#
# This script is meant for quick & easy install via:
#   'curl -sSL https://get.docker.com/ | sh'
# or:
#   'wget -qO- https://get.docker.com/ | sh'
#
#
# Docker Maintainers:
#   To update this script on https://get.docker.com,
#   use hack/release.sh during a normal release,
#   or the following one-liner for script hotfixes:
#     s3cmd put --acl-public -P hack/install.sh s3://get.docker.com/index
#

url='https://get.docker.com/'

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

user="$(id -un 2>/dev/null || true)"

sh_c='sh -c'
if [ "$user" != 'root' ]; then
	if command_exists sudo; then
		sh_c='sudo -E sh -c'
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
	curl='curl -sSL'
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
	lsb_dist='debian'
fi
if [ -z "$lsb_dist" ] && [ -r /etc/fedora-release ]; then
	lsb_dist='fedora'
fi
if [ -z "$lsb_dist" ] && [ -r /etc/os-release ]; then
	lsb_dist="$(. /etc/os-release && echo "$ID")"
fi

lsb_dist="$(echo "$lsb_dist" | tr '[:upper:]' '[:lower:]')"
case "$lsb_dist" in
	amzn|fedora)
		if [ "$lsb_dist" = 'amzn' ]; then
			(
				set -x
				$sh_c 'sleep 3; yum -y -q install docker'
			)
		else
			(
				set -x
				$sh_c 'sleep 3; yum -y -q install docker-io'
			)
		fi
		if command_exists docker && [ -e /var/run/docker.sock ]; then
			(
				set -x
				$sh_c 'docker version'
			) || true
		fi
		your_user=your-user
		[ "$user" != 'root' ] && your_user="$user"
		echo
		echo 'If you would like to use Docker as a non-root user, you should now consider'
		echo 'adding your user to the "docker" group with something like:'
		echo
		echo '  sudo usermod -aG docker' $your_user
		echo
		echo 'Remember that you will have to log out and back in for this to take effect!'
		echo
		exit 0
		;;

	ubuntu|debian|linuxmint)
		export DEBIAN_FRONTEND=noninteractive

		did_apt_get_update=
		apt_get_update() {
			if [ -z "$did_apt_get_update" ]; then
				( set -x; $sh_c 'sleep 3; apt-get update' )
				did_apt_get_update=1
			fi
		}

		# aufs is preferred over devicemapper; try to ensure the driver is available.
		if ! grep -q aufs /proc/filesystems && ! $sh_c 'modprobe aufs'; then
			kern_extras="linux-image-extra-$(uname -r)"

			apt_get_update
			( set -x; $sh_c 'sleep 3; apt-get install -y -q '"$kern_extras" ) || true

			if ! grep -q aufs /proc/filesystems && ! $sh_c 'modprobe aufs'; then
				echo >&2 'Warning: tried to install '"$kern_extras"' (for AUFS)'
				echo >&2 ' but we still have no AUFS.  Docker may not work. Proceeding anyways!'
				( set -x; sleep 10 )
			fi
		fi

		# install apparmor utils if they're missing and apparmor is enabled in the kernel
		# otherwise Docker will fail to start
		if [ "$(cat /sys/module/apparmor/parameters/enabled 2>/dev/null)" = 'Y' ]; then
			if command -v apparmor_parser &> /dev/null; then
				echo 'apparmor is enabled in the kernel and apparmor utils were already installed'
			else
				echo 'apparmor is enabled in the kernel, but apparmor_parser missing'
				apt_get_update
				( set -x; $sh_c 'sleep 3; apt-get install -y -q apparmor' )
			fi
		fi

		if [ ! -e /usr/lib/apt/methods/https ]; then
			apt_get_update
			( set -x; $sh_c 'sleep 3; apt-get install -y -q apt-transport-https' )
		fi
		if [ -z "$curl" ]; then
			apt_get_update
			( set -x; $sh_c 'sleep 3; apt-get install -y -q curl' )
			curl='curl -sSL'
		fi
		(
			set -x
			if [ "https://get.docker.com/" = "$url" ]; then
				$sh_c "apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9"
			elif [ "https://test.docker.com/" = "$url" ]; then
				$sh_c "apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 740B314AE3941731B942C66ADF4FD13717AAD7D6"
			else
				$sh_c "$curl ${url}gpg | apt-key add -"
			fi
			$sh_c "echo deb ${url}ubuntu docker main > /etc/apt/sources.list.d/docker.list"
			$sh_c 'sleep 3; apt-get update; apt-get install -y -q lxc-docker'
		)
		if command_exists docker && [ -e /var/run/docker.sock ]; then
			(
				set -x
				$sh_c 'docker version'
			) || true
		fi
		your_user=your-user
		[ "$user" != 'root' ] && your_user="$user"
		echo
		echo 'If you would like to use Docker as a non-root user, you should now consider'
		echo 'adding your user to the "docker" group with something like:'
		echo
		echo '  sudo usermod -aG docker' $your_user
		echo
		echo 'Remember that you will have to log out and back in for this to take effect!'
		echo
		exit 0
		;;

	gentoo)
		if [ "$url" = "https://test.docker.com/" ]; then
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

cat >&2 <<'EOF'

  Either your platform is not easily detectable, is not supported by this
  installer script (yet - PRs welcome! [hack/install.sh]), or does not yet have
  a package for Docker.  Please visit the following URL for more detailed
  installation instructions:

    https://docs.docker.com/en/latest/installation/

EOF
exit 1
