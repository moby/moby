#!/usr/bin/env bash
set -e

TESTFLAGS=${TESTFLAGS:-""}
# Currently only DockerSuite and DockerNetworkSuite have been adapted for E2E testing
TESTFLAGS_LEGACY=${TESTFLAGS_LEGACY:-""}
TIMEOUT=${TIMEOUT:-60m}

SCRIPTDIR="$(dirname ${BASH_SOURCE[0]})"

export DOCKER_ENGINE_GOARCH=${DOCKER_ENGINE_GOARCH:-amd64}

run_test_integration() {
  run_test_integration_suites
  run_test_integration_legacy_suites
}

run_test_integration_suites() {
  local flags="-test.v -test.timeout=${TIMEOUT} $TESTFLAGS"
  for dir in /tests/integration/*; do
    if ! (
      cd $dir
      echo "Running $PWD"
      ./test.main $flags
    ); then exit 1; fi
  done
}

run_test_integration_legacy_suites() {
  (
    flags="-check.v -check.timeout=${TIMEOUT} -test.timeout=360m $TESTFLAGS_LEGACY"
    cd /tests/integration-cli
    echo "Running $PWD"
    ./test.main $flags
  )
}

bash $SCRIPTDIR/ensure-emptyfs.sh

echo "Run integration tests"
run_test_integration
