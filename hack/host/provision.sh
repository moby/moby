#!/usr/bin/env bash

set -eux -o pipefail

# The argument is the login user that will run Docker in the guest.
mkdir -p /etc/systemd/system/docker.socket.d
cat <<- EOF | tee /etc/systemd/system/docker.socket.d/override.conf
	[Socket]
	SocketUser=$1
EOF
# TODO: use native packages for AlmaLinux: https://github.com/docker/packaging/pull/138
dnf config-manager --add-repo=https://download.docker.com/linux/rhel/docker-ce.repo
dnf -q -y install --nobest docker-ce make git
systemctl enable --now docker
