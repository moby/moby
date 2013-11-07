# VERSION:        0.1
# DESCRIPTION:    Create data image sharing /data volume
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# COMMENTS:
#   This image is used as base for all data containers.
#   /data volume is owned by sysadmin.
# USAGE:
#   # Download data Dockerfile
#   wget http://raw.github.com/dotcloud/docker/master/contrib/desktop-integration/data/Dockerfile
#
#   # Build data image
#   docker build -t data -rm .
#
#   # Create a data container. (eg: firefox-data)
#   docker run -name firefox-data data true
#
#   # List data from it
#   docker run -volumes-from firefox-data busybox ls -al /data

docker-version 0.6.5

# Smallest base image, just to launch a container
from busybox
maintainer	Daniel Mizyrycki <daniel@docker.com>

# Create a regular user
run echo 'sysadmin:x:1000:1000::/data:/bin/sh' >> /etc/passwd
run echo 'sysadmin:x:1000:' >> /etc/group

# Create directory for that user
run mkdir /data
run chown sysadmin.sysadmin /data

# Add content to /data. This will keep sysadmin ownership
run touch /data/init_volume

# Create /data volume
VOLUME /data
