#!/bin/sh -e
# This is a one time script to prepare docker-ci

# Build docker nightly release container
cd /go/src/github.com/dotcloud/docker/testing/nightlyrelease; docker build -t dockerbuilder .

# Self removing
echo -e '#!/bin/sh -e\nexit 0\n' > /etc/rc.local
exit 0
