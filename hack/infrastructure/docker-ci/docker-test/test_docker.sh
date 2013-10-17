#!/bin/bash

set -x
COMMIT=${1-HEAD}
REPO=${2-http://github.com/dotcloud/docker}
BRANCH=${3-master}

# Generate a random string of $1 characters
function random {
    cat /dev/urandom | tr -cd 'a-f0-9' | head -c $1
}

# Compute test paths
BASE_PATH=`pwd`/test_docker_$(random 12)
DOCKER_PATH=$BASE_PATH/go/src/github.com/dotcloud/docker
export GOPATH=$BASE_PATH/go:$DOCKER_PATH/vendor

# Fetch latest master
mkdir -p $DOCKER_PATH
cd $DOCKER_PATH
git init .
git fetch -q http://github.com/dotcloud/docker master
git reset --hard FETCH_HEAD

# Merge commit
git fetch -q "$REPO" "$BRANCH"
git merge --no-edit $COMMIT || exit 1

# Test commit
go test -v; exit_status=$?

# Cleanup testing directory
rm -rf $BASE_PATH

exit $exit_status
