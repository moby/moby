#!/bin/bash

set -x
COMMIT=${1-HEAD}
REPO=${2-http://github.com/dotcloud/docker}
BRANCH=${3-master}

# Compute test paths
DOCKER_PATH=/go/src/github.com/dotcloud/docker

# Fetch latest master
rm -rf /go
mkdir -p $DOCKER_PATH
cd $DOCKER_PATH
git init .
git fetch -q http://github.com/dotcloud/docker master
git reset --hard FETCH_HEAD

# Merge commit
#echo FIXME. Temporarily skip TestPrivilegedCanMount until DinD works reliable on AWS
git pull -q https://github.com/mzdaniel/docker.git dind-aws || exit 1

# Merge commit in top of master
git fetch -q "$REPO" "$BRANCH"
git merge --no-edit $COMMIT || exit 1

# Test commit
go test -v; exit_status=$?

# Display load if test fails
if [ $exit_status -eq 1 ] ; then
    uptime; echo; free
fi

# Cleanup testing directory
rm -rf $BASE_PATH

exit $exit_status
