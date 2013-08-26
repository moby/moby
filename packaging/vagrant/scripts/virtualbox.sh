#!/bin/sh

set -e

VBOX_VERSION=$(cat /home/vagrant/.vbox_version)
VBOX_ISO=VBoxGuestAdditions_$VBOX_VERSION.iso

# Abort if not on VirtualBox
[ -f "$VBOX_ISO" ] || exit 0

apt-get -y install dkms
mount -o loop "$VBOX_ISO" /mnt

# Installing the virtualbox guest additions
sh /mnt/VBoxLinuxAdditions.run --nox11 || true

umount /mnt
rm "$VBOX_ISO"
