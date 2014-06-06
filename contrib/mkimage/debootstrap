#!/usr/bin/env bash
set -e

rootfsDir="$1"
shift

# we have to do a little fancy footwork to make sure "rootfsDir" becomes the second non-option argument to debootstrap

before=()
while [ $# -gt 0 ] && [[ "$1" == -* ]]; do
	before+=( "$1" )
	shift
done

suite="$1"
shift

(
	set -x
	debootstrap "${before[@]}" "$suite" "$rootfsDir" "$@"
)

# now for some Docker-specific tweaks

# prevent init scripts from running during install/update
echo >&2 "+ cat > '$rootfsDir/usr/sbin/policy-rc.d'"
cat > "$rootfsDir/usr/sbin/policy-rc.d" <<'EOF'
#!/bin/sh
exit 101
EOF
chmod +x "$rootfsDir/usr/sbin/policy-rc.d"

# prevent upstart scripts from running during install/update
(
	set -x
	chroot "$rootfsDir" dpkg-divert --local --rename --add /sbin/initctl
	ln -sf /bin/true "$rootfsDir/sbin/initctl"
)

# shrink the image, since apt makes us fat (wheezy: ~157.5MB vs ~120MB)
( set -x; chroot "$rootfsDir" apt-get clean )

# Ubuntu 10.04 sucks... :)
if strings "$rootfsDir/usr/bin/dpkg" | grep -q unsafe-io; then
	# force dpkg not to call sync() after package extraction (speeding up installs)
	echo >&2 "+ echo force-unsafe-io > '$rootfsDir/etc/dpkg/dpkg.cfg.d/docker-apt-speedup'"
	echo 'force-unsafe-io' > "$rootfsDir/etc/dpkg/dpkg.cfg.d/docker-apt-speedup"
fi

if [ -d "$rootfsDir/etc/apt/apt.conf.d" ]; then
	# _keep_ us lean by effectively running "apt-get clean" after every install
	aptGetClean='"rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true";'
	echo >&2 "+ cat > '$rootfsDir/etc/apt/apt.conf.d/docker-clean'"
	cat > "$rootfsDir/etc/apt/apt.conf.d/docker-clean" <<-EOF
		DPkg::Post-Invoke { ${aptGetClean} };
		APT::Update::Post-Invoke { ${aptGetClean} };

		Dir::Cache::pkgcache "";
		Dir::Cache::srcpkgcache "";
	EOF

	# remove apt-cache translations for fast "apt-get update"
	echo >&2 "+ cat > '$rootfsDir/etc/apt/apt.conf.d/docker-no-languages'"
	echo 'Acquire::Languages "none";' > "$rootfsDir/etc/apt/apt.conf.d/docker-no-languages"
fi

if [ -z "$DONT_TOUCH_SOURCES_LIST" ]; then
	# tweak sources.list, where appropriate
	lsbDist=
	if [ -z "$lsbDist" -a -r "$rootfsDir/etc/os-release" ]; then
		lsbDist="$(. "$rootfsDir/etc/os-release" && echo "$ID")"
	fi
	if [ -z "$lsbDist" -a -r "$rootfsDir/etc/lsb-release" ]; then
		lsbDist="$(. "$rootfsDir/etc/lsb-release" && echo "$DISTRIB_ID")"
	fi
	if [ -z "$lsbDist" -a -r "$rootfsDir/etc/debian_version" ]; then
		lsbDist='Debian'
	fi
	case "$lsbDist" in
		debian|Debian)
			# updates and security!
			if [ "$suite" != 'sid' -a "$suite" != 'unstable' ]; then
				(
					set -x
					sed -i "p; s/ $suite main$/ ${suite}-updates main/" "$rootfsDir/etc/apt/sources.list"
					echo "deb http://security.debian.org $suite/updates main" >> "$rootfsDir/etc/apt/sources.list"
				)
			fi
			;;
		ubuntu|Ubuntu)
			# add the universe, updates, and security repositories
			(
				set -x
				sed -i "
					s/ $suite main$/ $suite main universe/; p;
					s/ $suite main/ ${suite}-updates main/; p;
					s/ $suite-updates main/ ${suite}-security main/
				" "$rootfsDir/etc/apt/sources.list"
			)
			;;
		tanglu|Tanglu)
			# add the updates repository
			if [ "$suite" != 'devel' ]; then
				(
					set -x
					sed -i "p; s/ $suite main$/ ${suite}-updates main/" "$rootfsDir/etc/apt/sources.list"
				)
			fi
			;;
		steamos|SteamOS)
			# add contrib and non-free
			(
				set -x
				sed -i "s/ $suite main$/ $suite main contrib non-free/" "$rootfsDir/etc/apt/sources.list"
			)
			;;
	esac
fi

# make sure we're fully up-to-date, too
(
	set -x
	chroot "$rootfsDir" apt-get update
	chroot "$rootfsDir" apt-get dist-upgrade -y
)
