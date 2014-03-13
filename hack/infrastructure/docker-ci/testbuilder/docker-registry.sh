#!/usr/bin/env bash
set -x
set -e
PROJECT_PATH=$1

# Build the docker project
cd /data/$PROJECT_PATH
sg docker -c "docker build -q -t registry ."
cd test; sg docker -c "docker build -q -t docker-registry-test ."

# Run the tests
sg docker -c "docker run --rm -v /home/docker-ci/coverage/docker-registry:/data docker-registry-test"
