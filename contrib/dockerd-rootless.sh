#!/bin/sh
# dockerd-rootless.sh executes dockerd in rootless mode.
#
# Usage: dockerd-rootless.sh [DOCKERD_OPTIONS]
#
# External dependencies:
# * newuidmap and newgidmap needs to be installed.
# * /etc/subuid and /etc/subgid needs to be configured for the current user.
# * Either slirp4netns (v0.3+) or VPNKit needs to be installed.
#
# See the documentation for the further information.

# The default flags used for running dockerd
: ${DOCKERD_ROOTLESS_DOCKERD_FLAGS:="--experimental --exec-opt native.cgroupdriver=none --exec-opt native.restrict_oom_score_adj=1 --userland-proxy --userland-proxy-path=$(which rootlesskit-docker-proxy)"}

set -e -x
if ! [ -w $XDG_RUNTIME_DIR ]; then
	echo "XDG_RUNTIME_DIR needs to be set and writable"
	exit 1
fi
if ! [ -w $HOME ]; then
	echo "HOME needs to be set and writable"
	exit 1
fi

rootlesskit=""
for f in docker-rootlesskit rootlesskit; do
	if which $f >/dev/null 2>&1; then
		rootlesskit=$f
		break
	fi
done
if [ -z $rootlesskit ]; then
	echo "rootlesskit needs to be installed"
	exit 1
fi

net=""
mtu=""
if which slirp4netns >/dev/null 2>&1; then
	if slirp4netns --help | grep -- --disable-host-loopback; then
		net=slirp4netns
		mtu=65520
	else
		echo "slirp4netns does not support --disable-host-loopback. Falling back to VPNKit."
	fi
fi
if [ -z $net ]; then
	if which vpnkit >/dev/null 2>&1; then
		net=vpnkit
		mtu=1500
	else
		echo "Either slirp4netns (v0.3+) or vpnkit needs to be installed"
		exit 1
	fi
fi

if [ -z $_DOCKERD_ROOTLESS_CHILD ]; then
	_DOCKERD_ROOTLESS_CHILD=1
	export _DOCKERD_ROOTLESS_CHILD
	# Re-exec the script via RootlessKit, so as to create unprivileged {user,mount,network} namespaces.
	#
	# --copy-up allows removing/creating files in the directories by creating tmpfs and symlinks
	# * /etc: copy-up is required so as to prevent `/etc/resolv.conf` in the
	#         namespace from being unexpectedly unmounted when `/etc/resolv.conf` is recreated on the host
	#         (by either systemd-networkd or NetworkManager)
	# * /run: copy-up is required so that we can create /run/docker (hardcoded for plugins) in our namespace
	$rootlesskit \
		--net=$net --mtu=$mtu --disable-host-loopback --port-driver=builtin \
		--copy-up=/etc --copy-up=/run \
		$DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS \
		$0 $DOCKERD_ROOTLESS_DOCKERD_FLAGS $@
else
	[ $_DOCKERD_ROOTLESS_CHILD = 1 ]
	# remove the symlinks for the existing files in the parent namespace if any,
	# so that we can create our own files in our mount namespace.
	rm -f /run/docker /run/xtables.lock
	DOCKER_HONOR_XDG=1
	export DOCKER_HONOR_XDG
	dockerd $@
fi
