#!/bin/sh

# This script builds the docker binary without using docker.
#
# It is meant to be used in situations where there is no option to run docker
# before the build.
#
# This version of the build is unsupported. It is your responsibility to ensure
# all dependencies are met and that the right version of go is used.
#
# Requirements:
# - The current directory should be a checkout of the docker source code
#   (http://github.com/dotcloud/docker). Whatever version is checked out
#   will be built.
# - The VERSION file, at the root of the repository, should exist, and
#   will be used as Docker binary version and package version.
# - The hash of the git commit will also be included in the Docker binary,
#   with the suffix -dirty if the repository isn't clean.
#

set -e

SCRIPT_DIR=`dirname "$0"`

# Load common code
source $SCRIPT_DIR/common.sh

# path of th project
PROJECT_DIR=`realpath $SCRIPT_DIR/../../`

# go path
export GOPATH=$PROJECT_DIR/.gopath

# source path
DOCKER_PACKAGE=github.com/dotcloud/docker
DOCKER_DIR=$GOPATH/src/$DOCKER_PACKAGE/docker

# target paths
BIN_DIR=$PROJECT_DIR/bin
DOCKER_BIN_RELATIVE=bin/docker
DOCKER_BIN=$PROJECT_DIR/$DOCKER_BIN_RELATIVE

setup_deps() {
        mkdir -p $GOPATH/src
        for i in $PROJECT_DIR/third_party/*; do
                if [ "$i" = "third_party/src" ]; then
                        continue
                fi
                cp -R "$i" $GOPATH/src
        done
}


prepare_docker() {
        if [ ! -h $GOPATH/src/$DOCKER_PACKAGE ]; then
                ln -s $PROJECT_DIR $GOPATH/src/$DOCKER_PACKAGE
        fi
}


main() {
        cat <<EOF
###############################################################################

 This version of the build is unsupported. It is your responsibility to ensure
 all dependencies are met and that the right version of go is used.

###############################################################################
EOF
        setup_deps
        prepare_docker
        cd $DOCKER_DIR
        CGO_ENABLED=0 go build -a -ldflags "$LDFLAGS" -o $DOCKER_BIN
        echo  $DOCKER_BIN_RELATIVE is created.
}

main
