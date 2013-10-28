#!/bin/bash
set -e

variant='minbase'
include='iproute,iputils-ping'
arch='amd64' # intentionally undocumented for now
skipDetection=
strictDebootstrap=
justTar=

usage() {
	echo >&2
	
	echo >&2 "usage: $0 [options] repo suite [mirror]"
	
	echo >&2
	echo >&2 'options: (not recommended)'
	echo >&2 "  -p set an http_proxy for debootstrap"
	echo >&2 "  -v $variant # change default debootstrap variant"
	echo >&2 "  -i $include # change default package includes"
	echo >&2 "  -d # strict debootstrap (do not apply any docker-specific tweaks)"
	echo >&2 "  -s # skip version detection and tagging (ie, precise also tagged as 12.04)"
	echo >&2 "     # note that this will also skip adding universe and/or security/updates to sources.list"
	echo >&2 "  -t # just create a tarball, especially for dockerbrew (uses repo as tarball name)"
	
	echo >&2
	echo >&2 "   ie: $0 username/debian squeeze"
	echo >&2 "       $0 username/debian squeeze http://ftp.uk.debian.org/debian/"
	
	echo >&2
	echo >&2 "   ie: $0 username/ubuntu precise"
	echo >&2 "       $0 username/ubuntu precise http://mirrors.melbourne.co.uk/ubuntu/"
	
	echo >&2
	echo >&2 "   ie: $0 -t precise.tar.bz2 precise"
	echo >&2 "       $0 -t wheezy.tgz wheezy"
	echo >&2 "       $0 -t wheezy-uk.tar.xz wheezy http://ftp.uk.debian.org/debian/"
	
	echo >&2
}

# these should match the names found at http://www.debian.org/releases/
debianStable=wheezy
debianUnstable=sid
# this should match the name found at http://releases.ubuntu.com/
ubuntuLatestLTS=precise

while getopts v:i:a:p:dst name; do
	case "$name" in
		p)
			http_proxy="$OPTARG"
			;;
		v)
			variant="$OPTARG"
			;;
		i)
			include="$OPTARG"
			;;
		a)
			arch="$OPTARG"
			;;
		d)
			strictDebootstrap=1
			;;
		s)
			skipDetection=1
			;;
		t)
			justTar=1
			;;
		?)
			usage
			exit 0
			;;
	esac
done
shift $(($OPTIND - 1))

repo="$1"
suite="$2"
mirror="${3:-}" # stick to the default debootstrap mirror if one is not provided

if [ ! "$repo" ] || [ ! "$suite" ]; then
	usage
	exit 1
fi

# some rudimentary detection for whether we need to "sudo" our docker calls
docker=''
if docker version > /dev/null 2>&1; then
	docker='docker'
elif sudo docker version > /dev/null 2>&1; then
	docker='sudo docker'
elif command -v docker > /dev/null 2>&1; then
	docker='docker'
else
	echo >&2 "warning: either docker isn't installed, or your current user cannot run it;"
	echo >&2 "         this script is not likely to work as expected"
	sleep 3
	docker='docker' # give us a command-not-found later
fi

# make sure we have an absolute path to our final tarball so we can still reference it properly after we change directory
if [ "$justTar" ]; then
	if [ ! -d "$(dirname "$repo")" ]; then
		echo >&2 "error: $(dirname "$repo") does not exist"
		exit 1
	fi
	repo="$(cd "$(dirname "$repo")" && pwd -P)/$(basename "$repo")"
fi

# will be filled in later, if [ -z "$skipDetection" ]
lsbDist=''

target="/tmp/docker-rootfs-debootstrap-$suite-$$-$RANDOM"

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"
returnTo="$(pwd -P)"

set -x

# bootstrap
mkdir -p "$target"
sudo http_proxy=$http_proxy debootstrap --verbose --variant="$variant" --include="$include" --arch="$arch" "$suite" "$target" "$mirror"

cd "$target"

if [ -z "$strictDebootstrap" ]; then
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
	
	if [ -z "$skipDetection" ]; then
		# see also rudimentary platform detection in hack/install.sh
		lsbDist=''
		if [ -r etc/lsb-release ]; then
			lsbDist="$(. etc/lsb-release && echo "$DISTRIB_ID")"
		fi
		if [ -z "$lsbDist" ] && [ -r etc/debian_version ]; then
			lsbDist='Debian'
		fi
		
		case "$lsbDist" in
			Debian)
				# add the updates and security repositories
				if [ "$suite" != "$debianUnstable" -a "$suite" != 'unstable' ]; then
					# ${suite}-updates only applies to non-unstable
					sudo sed -i "p; s/ $suite main$/ ${suite}-updates main/" etc/apt/sources.list
					
					# same for security updates
					echo "deb http://security.debian.org/ $suite/updates main" | sudo tee -a etc/apt/sources.list > /dev/null
				fi
				;;
			Ubuntu)
				# add the universe, updates, and security repositories
				sudo sed -i "
					s/ $suite main$/ $suite main universe/; p;
					s/ $suite main/ ${suite}-updates main/; p;
					s/ $suite-updates main/ ${suite}-security main/
				" etc/apt/sources.list
				;;
		esac
	fi
fi

if [ "$justTar" ]; then
	# create the tarball file so it has the right permissions (ie, not root)
	touch "$repo"
	
	# fill the tarball
	sudo tar --numeric-owner -caf "$repo" .
else
	# create the image (and tag $repo:$suite)
	sudo tar --numeric-owner -c . | $docker import - $repo $suite
	
	# test the image
	$docker run -i -t $repo:$suite echo success
	
	if [ -z "$skipDetection" ]; then
		case "$lsbDist" in
			Debian)
				if [ "$suite" = "$debianStable" -o "$suite" = 'stable' ] && [ -r etc/debian_version ]; then
					# tag latest
					$docker tag $repo:$suite $repo latest
					
					if [ -r etc/debian_version ]; then
						# tag the specific debian release version (which is only reasonable to tag on debian stable)
						ver=$(cat etc/debian_version)
						$docker tag $repo:$suite $repo $ver
					fi
				fi
				;;
			Ubuntu)
				if [ "$suite" = "$ubuntuLatestLTS" ]; then
					# tag latest
					$docker tag $repo:$suite $repo latest
				fi
				if [ -r etc/lsb-release ]; then
					lsbRelease="$(. etc/lsb-release && echo "$DISTRIB_RELEASE")"
					if [ "$lsbRelease" ]; then
						# tag specific Ubuntu version number, if available (12.04, etc.)
						$docker tag $repo:$suite $repo $lsbRelease
					fi
				fi
				;;
		esac
	fi
fi

# cleanup
cd "$returnTo"
sudo rm -rf "$target"
