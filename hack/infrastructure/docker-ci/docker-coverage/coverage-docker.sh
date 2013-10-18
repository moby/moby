#!/bin/bash

set -x
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

# Fetch go coverage
cd $BASE_PATH/go
GOPATH=$BASE_PATH/go go get github.com/axw/gocov/gocov
sudo -E GOPATH=$GOPATH ./bin/gocov test -deps -exclude-goroot -v\
 -exclude github.com/gorilla/context,github.com/gorilla/mux,github.com/kr/pty,\
code.google.com/p/go.net/websocket,github.com/dotcloud/tar\
 github.com/dotcloud/docker | ./bin/gocov report; exit_status=$?

# Cleanup testing directory
rm -rf $BASE_PATH

exit $exit_status
