#!/usr/bin/env bash
set -e

echo "Ensure emptyfs image is loaded"
bash /ensure-emptyfs.sh

echo "Run integration/container tests"
cd /tests/integration/container
./test.main -test.v

echo "Run integration-cli DockerSuite tests"
cd /tests/integration-cli
./test.main -test.v -check.v -check.f DockerSuite
