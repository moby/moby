#!/bin/sh
# This script is meant for quick & easy install via 'curl URL-OF-SCRIPT | sh'
# Original version by Jeff Lindsay <progrium@gmail.com>
# Revamped by Jerome Petazzoni <jerome@dotcloud.com>
#
# This script canonical location is https://get.docker.io/; to update it, run:
# s3cmd put -m text/x-shellscript -P install.sh s3://get.docker.io/index

echo "Ensuring basic dependencies are installed..."
apt-get -qq update
apt-get -qq install lxc wget

echo "Looking in /proc/filesystems to see if we have AUFS support..."
if grep -q aufs /proc/filesystems
then
    echo "Found."
else
    echo "Ahem, it looks like the current kernel does not support AUFS."
    echo "Let's see if we can load the AUFS module with modprobe..."
    if modprobe aufs
    then
        echo "Module loaded."
    else
        echo "Ahem, things didn't turn out as expected."
        KPKG=linux-image-extra-$(uname -r)
        echo "Trying to install $KPKG..."
        if apt-get -qq install $KPKG
        then
            echo "Installed."
        else
            echo "Oops, we couldn't install the -extra kernel."
            echo "Are you sure you are running a supported version of Ubuntu?"
            echo "Proceeding anyway, but Docker will probably NOT WORK!"
        fi
    fi
fi

echo "Downloading docker binary and uncompressing into /usr/local/bin..."
curl -s https://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-latest.tgz |
tar -C /usr/local/bin --strip-components=1 -zxf- \
docker-latest/docker

if [ -f /etc/init/dockerd.conf ]
then
  echo "Upstart script already exists."
else
  echo "Creating /etc/init/dockerd.conf..."
  cat >/etc/init/dockerd.conf <<EOF
description "Docker daemon"
start on filesystem or runlevel [2345]
stop on runlevel [!2345]
respawn
exec env LANG="en_US.UTF-8" /usr/local/bin/docker -d
EOF
fi

echo "Starting dockerd..."
start dockerd > /dev/null

echo "Done."
echo
