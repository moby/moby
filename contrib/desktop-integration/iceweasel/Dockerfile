# VERSION:        0.7
# DESCRIPTION:    Create iceweasel container with its dependencies
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# COMMENTS:
#   This file describes how to build a Iceweasel container with all
#   dependencies installed. It uses native X11 unix socket and alsa
#   sound devices. Tested on Debian 7.2
# USAGE:
#   # Download Iceweasel Dockerfile
#   wget http://raw.githubusercontent.com/dotcloud/docker/master/contrib/desktop-integration/iceweasel/Dockerfile
#
#   # Build iceweasel image
#   docker build -t iceweasel .
#
#   # Run stateful data-on-host iceweasel. For ephemeral, remove -v /data/iceweasel:/data
#   docker run -v /data/iceweasel:/data -v /tmp/.X11-unix:/tmp/.X11-unix \
#     -v /dev/snd:/dev/snd --lxc-conf='lxc.cgroup.devices.allow = c 116:* rwm' \
#     -e DISPLAY=unix$DISPLAY iceweasel
#
#   # To run stateful dockerized data containers
#   docker run --volumes-from iceweasel-data -v /tmp/.X11-unix:/tmp/.X11-unix \
#     -v /dev/snd:/dev/snd --lxc-conf='lxc.cgroup.devices.allow = c 116:* rwm' \
#     -e DISPLAY=unix$DISPLAY iceweasel

docker-version 0.6.5

# Base docker image
FROM debian:wheezy
MAINTAINER Daniel Mizyrycki <daniel@docker.com>

# Install Iceweasel and "sudo"
RUN apt-get update && apt-get install -y iceweasel sudo

# create sysadmin account
RUN useradd -m -d /data -p saIVpsc0EVTwA sysadmin
RUN sed -Ei 's/sudo:x:27:/sudo:x:27:sysadmin/' /etc/group
RUN sed -Ei 's/(\%sudo\s+ALL=\(ALL\:ALL\) )ALL/\1 NOPASSWD:ALL/' /etc/sudoers

# Autorun iceweasel. -no-remote is necessary to create a new container, as
# iceweasel appears to communicate with itself through X11.
CMD ["/usr/bin/sudo", "-u", "sysadmin", "-H", "-E", "/usr/bin/iceweasel", "-no-remote"]
