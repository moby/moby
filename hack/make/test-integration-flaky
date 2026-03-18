#!/usr/bin/env bash
set -e -o pipefail

source hack/validate/.validate

run_integration_flaky() {
	new_tests=$(
		validate_diff --diff-filter=ACMR --unified=0 -- 'integration/*_test.go' \
			| grep -E '^(\+func Test)(.*)(\*testing\.T\))' || true
	)

	if [ -z "$new_tests" ]; then
		echo 'No new tests added to integration.'
		return
	fi

	echo
	echo "Found new integrations tests:"
	echo "$new_tests"
	echo "Running stress test for them."

	(
		TESTARRAY=$(echo "$new_tests" | sed 's/+func //' | awk -F'\\(' '{print $1}' | tr '\n' '|')
		# Note: TEST_REPEAT will make the test suite run 5 times, restarting the daemon
		# and each test will run 5 times in a row under the same daemon.
		# This will make a total of 25 runs for each test in TESTARRAY.
		export TEST_REPEAT=5
		export TESTFLAGS="-test.count ${TEST_REPEAT} -test.run ${TESTARRAY%?}"
		echo "Using test flags: $TESTFLAGS"
		source hack/make/test-integration
	)
}

run_integration_flaky
