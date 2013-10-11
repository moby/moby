#!/bin/bash

# This script prepares the docker images to run tests efficiently.
#
# Requirements: 
# - jq http://stedolan.github.io/jq/ for parsing out json

set -e

DOCKER="sudo docker"

# do the build as normal
$DOCKER build -t docker .

# save the config
config=$($DOCKER inspect docker | jq .[0].config)

id=$($DOCKER run -d -dns=8.8.8.8 -lxc-conf=lxc.aa_profile=unconfined -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh binary test_prepare)
$DOCKER attach $id

# update the config with the id, so that the volumes are copied
config=$(echo $config | jq ".VolumesFrom=\"$id\" | .Volumes=null")

# commit a new image with the existing config
$DOCKER commit -run="$config" $id docker-test
