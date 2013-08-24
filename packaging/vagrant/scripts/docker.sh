#!/bin/sh

set -e

curl https://get.docker.io/gpg | apt-key add -
echo deb http://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list
apt-get update

apt-get install -qy lxc-docker

# Enable more cgroup features
sed -e 's/GRUB_CMDLINE_LINUX=""/GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"/g' -i /etc/default/grub
update-grub
