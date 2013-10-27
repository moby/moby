#!/bin/bash

set -x

# Compute test paths
REGISTRY_PATH=/data/docker-registry

# Fetch latest docker-registry master
rm -rf $REGISTRY_PATH
git clone https://github.com/dotcloud/docker-registry -b master $REGISTRY_PATH
cd $REGISTRY_PATH

# Generate coverage
export SETTINGS_FLAVOR=test
export DOCKER_REGISTRY_CONFIG=config_test.yml

coverage run -m unittest discover test || exit 1
coverage report --include='./*' --omit='./test/*'
