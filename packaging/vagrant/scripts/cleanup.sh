#!/bin/sh

set -e

# Remove items used for building, since they aren't needed anymore
echo "*** removing unnessary packages"
aptitude purge ~c
apt-get -y autoremove
apt-get clean

# Removing leftover leases and persistent rules
echo "*** cleaning up dhcp leases"
rm -f /var/lib/dhcp/*

# Make sure Udev doesn't block our network
# http://6.ptmc.org/?p=164
echo "*** cleaning up udev rules"
rm /etc/udev/rules.d/70-persistent-net.rules
mkdir /etc/udev/rules.d/70-persistent-net.rules
rm -rf /dev/.udev/
rm /lib/udev/rules.d/75-persistent-net-generator.rules

echo "Adding a 2 sec delay to the interface up, to make the dhclient happy"
echo "pre-up sleep 2" >> /etc/network/interfaces
