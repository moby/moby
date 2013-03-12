#!/bin/sh
# This script is meant for quick & easy install via 'curl URL-OF-SCRIPT | bash'
# Courtesy of Jeff Lindsay <progrium@gmail.com>

echo "Ensuring dependencies are installed..."
apt-get --yes install lxc wget bsdtar 2>&1 > /dev/null

echo "Downloading docker binary and uncompressing into /usr/local/bin..."
curl -s http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz |
tar -C /usr/local/bin --strip-components=1 -zxf- \
docker-master/docker docker-master/dockerd

if [[ -f /etc/init/dockerd.conf ]]
then
  echo "Upstart script already exists."
else
  echo "Creating /etc/init/dockerd.conf..."
  echo "exec /usr/local/bin/dockerd" > /etc/init/dockerd.conf
fi

echo "Starting dockerd..."
start dockerd > /dev/null

echo "Finished!"
echo
