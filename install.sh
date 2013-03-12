#!/bin/sh
# This script is meant for quick & easy install via 'curl URL-OF-SCRIPT | sh'
# Original version by Jeff Lindsay <progrium@gmail.com>
# Revamped by Jerome Petazzoni <jerome@dotcloud.com>
#
# This script canonical location is http://get.docker.io/; to update it, run:
# s3cmd put -m text/x-shellscript -P install.sh s3://get.docker.io/index

echo "Ensuring basic dependencies are installed..."
apt-get -qq update
apt-get -qq install lxc wget bsdtar

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
curl -s http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz |
tar -C /usr/local/bin --strip-components=1 -zxf- \
docker-master/docker docker-master/dockerd

if [ -f /etc/init/dockerd.conf ]
then
  echo "Upstart script already exists."
else
  echo "Creating /etc/init/dockerd.conf..."
  echo "exec /usr/local/bin/dockerd" > /etc/init/dockerd.conf
fi

echo "Starting dockerd..."
start dockerd > /dev/null

echo "Done."
echo
