#!/usr/bin/env bash

# Provision a dedicated host for integration tests. Run as root.
# RHEL-compatible and SUSE-compatible hosts are supported.
set -eux -o pipefail

packages=(
	gcc pkgconf-pkg-config systemd-devel
	iptables iputils nftables libselinux-utils
	fuse-overlayfs slirp4netns
)

if command -v dnf > /dev/null 2>&1; then
	packages+=(git iproute procps-ng shadow-utils)
	dnf -q -y install "${packages[@]}"
elif command -v zypper > /dev/null 2>&1; then
	packages+=(git-core iproute2 procps shadow)
	zypper -q --non-interactive install --no-recommends "${packages[@]}"
else
	echo "Unsupported distribution: neither dnf nor zypper was found" >&2
	exit 1
fi

# The integration test helpers use this account for rootless daemons.
# Lima reserves a large subordinate ID range for its login user.
useradd --create-home \
	--key SUB_UID_MAX=2147483647 --key SUB_GID_MAX=2147483647 \
	unprivilegeduser
mkdir -p /etc/systemd/system/user@.service.d
cat << EOF > /etc/systemd/system/user@.service.d/delegate.conf
[Service]
Delegate=cpu cpuset io memory pids
EOF
systemctl daemon-reload
