#!/bin/bash

# Setup of test credentials. Called by Vagrantfile
export PATH="/bin:sbin:/usr/bin:/usr/sbin:/usr/local/bin"

USER=$1
REGISTRY_USER=$2
REGISTRY_PWD=$3

BUILDBOT_PATH="/data/buildbot"
DOCKER_PATH="/data/docker"

function run { su $USER -c "$1"; }

run "cp $DOCKER_PATH/testing/buildbot/credentials.cfg $BUILDBOT_PATH/master"
cd $BUILDBOT_PATH/master
run "sed -i -E 's#(export DOCKER_CREDS=).+#\1\"$REGISTRY_USER:$REGISTRY_PWD\"#' credentials.cfg"
