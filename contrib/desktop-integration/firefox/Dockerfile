# VERSION:        0.7
# DESCRIPTION:    Create firefox container with its dependencies
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# COMMENTS:
#   This file describes how to build a Firefox container with all
#   dependencies installed. It uses native X11 unix socket and alsa
#   sound devices. Tested on Debian 7.2
# USAGE:
#   # Download Firefox Dockerfile
#   wget http://raw.github.com/dotcloud/docker/master/contrib/desktop-integration/firefox/Dockerfile
#
#   # Build firefox image
#   docker build -t firefox -rm .
#
#   # Run stateful data-on-host firefox. For ephemeral, remove -v /data/firefox:/data
#   docker run -v /data/firefox:/data -v /tmp/.X11-unix:/tmp/.X11-unix \
#     -v /dev/snd:/dev/snd -lxc-conf='lxc.cgroup.devices.allow = c 116:* rwm' \
#     -e DISPLAY=unix$DISPLAY firefox
#
#   # To run stateful dockerized data containers
#   docker run -volumes-from firefox-data -v /tmp/.X11-unix:/tmp/.X11-unix \
#     -v /dev/snd:/dev/snd -lxc-conf='lxc.cgroup.devices.allow = c 116:* rwm' \
#     -e DISPLAY=unix$DISPLAY firefox

docker-version 0.6.5

# Base docker image
from tianon/debian:wheezy
maintainer	Daniel Mizyrycki <daniel@docker.com>

# Install firefox dependencies
run echo "deb http://ftp.debian.org/debian/ wheezy main contrib" > /etc/apt/sources.list
run apt-get update
run DEBIAN_FRONTEND=noninteractive apt-get install -y libXrender1 libasound2 \
    libdbus-glib-1-2 libgtk2.0-0 libpango1.0-0 libxt6 wget bzip2 sudo

# Install Firefox
run mkdir /application
run cd /application; wget -O - \
    http://ftp.mozilla.org/pub/mozilla.org/firefox/releases/25.0/linux-x86_64/en-US/firefox-25.0.tar.bz2 | tar jx

# create sysadmin account
run useradd -m -d /data -p saIVpsc0EVTwA sysadmin
run sed -Ei 's/sudo:x:27:/sudo:x:27:sysadmin/' /etc/group
run sed -Ei 's/(\%sudo\s+ALL=\(ALL\:ALL\) )ALL/\1 NOPASSWD:ALL/' /etc/sudoers

# Autorun firefox. -no-remote is necessary to create a new container, as firefox
# appears to communicate with itself through X11.
cmd ["/bin/sh", "-c", "/usr/bin/sudo -u sysadmin -H -E /application/firefox/firefox -no-remote"]
