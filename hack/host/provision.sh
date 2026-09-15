#!/usr/bin/env bash

# Provision a dedicated RHEL-compatible host for integration tests. Run as root.
set -eux -o pipefail

dnf -q -y install \
	gcc git pkgconf-pkg-config systemd-devel \
	iproute iptables iputils nftables procps-ng libselinux-utils \
	fuse-overlayfs slirp4netns shadow-utils

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
