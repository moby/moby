#!/usr/bin/env bash
set -e -u -o pipefail

ARCH=$(uname -m)
if [ "$ARCH" == "x86_64" ]; then
  ARCH="amd64"
fi

export DOCKER_ENGINE_GOARCH=${DOCKER_ENGINE_GOARCH:-${ARCH}}

# Set defaults
: ${TESTFLAGS:=}
: ${TESTDEBUG:=}

integration_api_dirs=${TEST_INTEGRATION_DIR:-"$(
	find /tests/integration -type d |
	grep -vE '(^/tests/integration($|/internal)|/testdata)')"}

run_test_integration() {
	[[ "$TESTFLAGS" != *-check.f* ]] && run_test_integration_suites
	run_test_integration_legacy_suites
}

run_test_integration_suites() {
	local flags="-test.v -test.timeout=${TIMEOUT:=10m} $TESTFLAGS"
	for dir in $integration_api_dirs; do
		if ! (
			cd $dir
			echo "Running $PWD"
			test_env ./test.main $flags
		); then exit 1; fi
	done
}

run_test_integration_legacy_suites() {
	(
		flags="-check.v -check.timeout=${TIMEOUT} -test.timeout=360m $TESTFLAGS"
		cd /tests/integration-cli
		echo "Running $PWD"
		test_env ./test.main $flags
	)
}

# use "env -i" to tightly control the environment variables that bleed into the tests
test_env() {
	(
		set -e +u
		[ -n "$TESTDEBUG" ] && set -x
		env -i \
			DOCKER_API_VERSION="$DOCKER_API_VERSION" \
			DOCKER_INTEGRATION_DAEMON_DEST="$DOCKER_INTEGRATION_DAEMON_DEST" \
			DOCKER_TLS_VERIFY="$DOCKER_TEST_TLS_VERIFY" \
			DOCKER_CERT_PATH="$DOCKER_TEST_CERT_PATH" \
			DOCKER_ENGINE_GOARCH="$DOCKER_ENGINE_GOARCH" \
			DOCKER_GRAPHDRIVER="$DOCKER_GRAPHDRIVER" \
			DOCKER_USERLANDPROXY="$DOCKER_USERLANDPROXY" \
			DOCKER_HOST="$DOCKER_HOST" \
			DOCKER_REMAP_ROOT="$DOCKER_REMAP_ROOT" \
			DOCKER_REMOTE_DAEMON="$DOCKER_REMOTE_DAEMON" \
			DOCKERFILE="$DOCKERFILE" \
			GOPATH="$GOPATH" \
			GOTRACEBACK=all \
			HOME="$ABS_DEST/fake-HOME" \
			PATH="$PATH" \
			TEMP="$TEMP" \
			TEST_CLIENT_BINARY="$TEST_CLIENT_BINARY" \
			"$@"
	)
}

run_test_integration
