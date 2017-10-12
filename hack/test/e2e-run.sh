#!/usr/bin/env bash
set -e

TESTFLAGS=${TESTFLAGS:-""}
# Currently only DockerSuite and DockerNetworkSuite have been adapted for E2E testing
TESTFLAGS_LEGACY=${TESTFLAGS_LEGACY:-""}
TIMEOUT=${TIMEOUT:-60m}

SCRIPTDIR="$(dirname ${BASH_SOURCE[0]})"

if [ $(uname -m) == "x86_64" ]; then
  ARCH="amd64"
else
  ARCH=$(uname -m)
fi

export DOCKER_ENGINE_GOARCH=${DOCKER_ENGINE_GOARCH:-${ARCH}}

run_test_integration() {
  run_test_integration_suites
  run_test_integration_legacy_suites
}

run_test_integration_suites() {
  local flags="-test.timeout=${TIMEOUT} $TESTFLAGS"
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
    flags="-check.timeout=${TIMEOUT} -test.timeout=360m $TESTFLAGS_LEGACY"
    cd /tests/integration-cli
    echo "Running $PWD"
    ./test.main $flags
  )
}

bash $SCRIPTDIR/ensure-emptyfs.sh

echo "ARCH: ${ARCH}"
echo "DOCKER_ENGINE_GOARCH: ${DOCKER_ENGINE_GOARCH}"
echo "DOCKER_INTEGRATION_DAEMON_DEST: ${DOCKER_INTEGRATION_DAEMON_DEST}"
echo "DOCKER_REMOTE_DAEMON: ${DOCKER_REMOTE_DAEMON}"
echo "TESTFLAGS: ${TESTFLAGS}"
echo "TESTFLAGS_LEGACY: ${TESTFLAGS_LEGACY}"
echo "TIMEOUT: ${TIMEOUT}"
echo ""
echo "Run integration tests"

run_test_integration
