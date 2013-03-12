#!/bin/sh
# This script is meant for quick & easy install via 'curl URL-OF-SCRIPT | bash'
# Courtesy of Jeff Lindsay <progrium@gmail.com>

cd /tmp

echo "Ensuring dependencies are installed..."
apt-get --yes install lxc wget bsdtar 2>&1 > /dev/null

echo "Downloading docker binary..."
wget -q https://dl.dropbox.com/u/20637798/docker.tar.gz 2>&1 > /dev/null
tar -xf docker.tar.gz 2>&1 > /dev/null

echo "Installing into /usr/local/bin..."
mv docker/docker /usr/local/bin
mv dockerd/dockerd /usr/local/bin

if [[ -f /etc/init/dockerd.conf ]]
then
  echo "Upstart script already exists."
else
  echo "Creating /etc/init/dockerd.conf..."
  echo "exec /usr/local/bin/dockerd" > /etc/init/dockerd.conf
fi

echo "Restarting dockerd..."
restart dockerd > /dev/null

echo "Cleaning up..."
rmdir docker
rmdir dockerd
rm docker.tar.gz

echo "Finished!"
echo
