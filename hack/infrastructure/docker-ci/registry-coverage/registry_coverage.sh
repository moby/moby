#!/bin/bash

set -x

# Setup the environment
REGISTRY_PATH=/data/docker-registry
export SETTINGS_FLAVOR=test
export DOCKER_REGISTRY_CONFIG=config_test.yml
export PYTHONPATH=$REGISTRY_PATH/test

# Fetch latest docker-registry master
rm -rf $REGISTRY_PATH
git clone https://github.com/dotcloud/docker-registry -b master $REGISTRY_PATH
cd $REGISTRY_PATH

# Generate coverage
coverage run -m unittest discover test || exit 1
coverage report --include='./*' --omit='./test/*'
