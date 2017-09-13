#!/usr/bin/env bash
set -e

export DOCKER_ENGINE_GOARCH=${DOCKER_ENGINE_GOARCH:-amd64}

echo "Ensure emptyfs image is loaded"
bash /ensure-emptyfs.sh

echo "Run integration/container tests"
cd /tests/integration/container
./test.main -test.v

echo "Run integration-cli tests (DockerSuite, DockerNetworkSuite)"
cd /tests/integration-cli
./test.main -test.v -check.v -check.f "DockerSuite|DockerNetworkSuite"
