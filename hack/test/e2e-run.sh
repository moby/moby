#!/usr/bin/env bash
set -e -u -o pipefail

# Set defaults
: ${TESTFLAGS:=}
: ${TESTDEBUG:=}

integration_api_dirs=${TEST_INTEGRATION_DIR:-"$(
	find /tests/integration -type d \
		| grep -vE '(^/tests/integration($|/internal)|/testdata)'
)"}

run_test_integration() {
	run_test_integration_suites
	run_test_integration_legacy_suites
}

run_test_integration_suites() {
	local flags="-test.v -test.timeout=${TIMEOUT} $TESTFLAGS"
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
		flags="-test.v -test.timeout=360m $TESTFLAGS"
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
			DOCKER_GRAPHDRIVER="$DOCKER_GRAPHDRIVER" \
			DOCKER_USERLANDPROXY="$DOCKER_USERLANDPROXY" \
			DOCKER_HOST="$DOCKER_HOST" \
			DOCKER_REMAP_ROOT="$DOCKER_REMAP_ROOT" \
			DOCKER_REMOTE_DAEMON="$DOCKER_REMOTE_DAEMON" \
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
