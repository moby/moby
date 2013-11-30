#!/bin/bash

set -x
COMMIT=${1-HEAD}
REPO=${2-http://github.com/dotcloud/docker}
BRANCH=${3-master}

# Compute test paths
DOCKER_PATH=/go/src/github.com/dotcloud/docker

# Timestamp
echo
date; echo

# Fetch latest master
cd /
rm -rf /go
git clone -q -b master http://github.com/dotcloud/docker $DOCKER_PATH
cd $DOCKER_PATH

# Merge commit
git fetch -q "$REPO" "$BRANCH"
git merge --no-edit $COMMIT || exit 255

# Test commit
./hack/make.sh test; exit_status=$?

# Display load if test fails
if [ $exit_status -ne 0 ] ; then
    uptime; echo; free
fi

exit $exit_status
