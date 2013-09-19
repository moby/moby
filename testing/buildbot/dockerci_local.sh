#!/bin/sh -e
# This is a one time script to prepare docker-ci

# Build docker nightly release container
cd /go/src/github.com/dotcloud/docker/testing/nightlyrelease; docker build -t dockerbuilder .

# Relaunch docker for dind to work (disabling apparmor)
/sbin/stop docker
DIND_CMD="    /etc/init.d/apparmor stop; /etc/init.d/apparmor teardown; /usr/bin/docker -dns=8.8.8.8 -d"
sed -Ei "s~    /usr/bin/docker -d~$DIND_CMD~" /etc/init/docker.conf
/sbin/start docker

# Self removing
echo -e '#!/bin/sh -e\nexit 0\n' > /etc/rc.local
exit 0
