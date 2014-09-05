# VERSION:        0.1
# DESCRIPTION:    Create data image sharing /data volume
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# COMMENTS:
#   This image is used as base for all data containers.
#   /data volume is owned by sysadmin.
# USAGE:
#   # Download data Dockerfile
#   wget http://raw.githubusercontent.com/dotcloud/docker/master/contrib/desktop-integration/data/Dockerfile
#
#   # Build data image
#   docker build -t data .
#
#   # Create a data container. (eg: iceweasel-data)
#   docker run --name iceweasel-data data true
#
#   # List data from it
#   docker run --volumes-from iceweasel-data busybox ls -al /data

docker-version 0.6.5

# Smallest base image, just to launch a container
FROM busybox
MAINTAINER Daniel Mizyrycki <daniel@docker.com>

# Create a regular user
RUN echo 'sysadmin:x:1000:1000::/data:/bin/sh' >> /etc/passwd
RUN echo 'sysadmin:x:1000:' >> /etc/group

# Create directory for that user
RUN mkdir /data
RUN chown sysadmin.sysadmin /data

# Add content to /data. This will keep sysadmin ownership
RUN touch /data/init_volume

# Create /data volume
VOLUME /data
