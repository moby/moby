# Testing

This document contains the Moby code testing guidelines. It should answer any 
questions you may have as an aspiring Moby contributor.

## Test suites

Moby has two test suites (and one legacy test suite):

* Unit tests - use standard `go test` and
  [gotest.tools/v3/assert](https://pkg.go.dev/gotest.tools/v3/assert) assertions. They are located in
  the package they test. Unit tests should be fast and test only their own 
  package.
* API integration tests - use standard `go test` and
  [gotest.tools/v3/assert](https://pkg.go.dev/gotest.tools/v3/assert) assertions. They are located in
  `./integration/<component>` directories, where `component` is: container,
  image, volume, etc. These tests perform HTTP requests to an API endpoint and
  check the HTTP response and daemon state after the call.

The legacy test suite `integration-cli/` is deprecated. No new tests will be 
added to this suite. Any tests in this suite which require updates should be 
ported to either the unit test suite or the new API integration test suite.

## Writing new tests

Most code changes will fall into one of the following categories.

### Writing tests for new features

New code should be covered by unit tests. If the code is difficult to test with
unit tests, then that is a good sign that it should be refactored to make it
easier to reuse and maintain. Consider accepting unexported interfaces instead
of structs so that fakes can be provided for dependencies.

If the new feature includes a completely new API endpoint then a new API 
integration test should be added to cover the success case of that endpoint.

If the new feature does not include a completely new API endpoint consider 
adding the new API fields to the existing test for that endpoint. A new 
integration test should **not** be added for every new API field or API error 
case. Error cases should be handled by unit tests.

### Writing tests for bug fixes

Bugs fixes should include a unit test case which exercises the bug.

A bug fix may also include new assertions in existing integration tests for the
API endpoint.

### Writing new integration tests

Note the `integration-cli` tests are deprecated; new tests will be rejected by
the CI.

Instead, implement new tests under `integration/`.

### Integration tests environment considerations

When adding new tests or modifying existing tests under `integration/`, testing
environment should be properly considered. [`skip.If`](https://pkg.go.dev/gotest.tools/v3/skip#If) from 
[gotest.tools/v3/skip](https://pkg.go.dev/gotest.tools/v3/skip) can be used to make the 
test run conditionally. Full testing environment conditions can be found at 
[environment.go](https://github.com/moby/moby/blob/311b2c87e125c6d4198014369e313135cf928a8a/testutil/environment/environment.go)

Here is a quick example. If the test needs to interact with a docker daemon on 
the same host, the following condition should be checked within the test code

```go
package example

import (
	"testing"

	"gotest.tools/v3/skip"
)

func TestSomething(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon(), "test requires a local daemon")

	// your integration test code
}
```

If a remote daemon is detected, the test will be skipped.

## Running tests

### Unit Tests

To run the unit test suite:

```bash
make test-unit
```

or `hack/test/unit` from inside a `make shell` container or properly
configured environment.

The following environment variables may be used to run a subset of tests:

* `TESTDIRS` - paths to directories to be tested, defaults to `./...`
* `TESTFLAGS` - flags passed to `go test`, to run tests which match a pattern
  use `TESTFLAGS="-test.run TestNameOrPrefix"`

### Integration Tests

To run the integration test suite:

```bash
make test-integration
```

This make target runs both the "integration" suite and the "integration-cli"
suite.

You can specify which integration test dirs to build and run by specifying
the list of dirs in the TEST_INTEGRATION_DIR environment variable.

You can also explicitly skip either suite by setting (any value) in
TEST_SKIP_INTEGRATION and/or TEST_SKIP_INTEGRATION_CLI environment variables.

Flags specific to each suite can be set in the TESTFLAGS_INTEGRATION and
TESTFLAGS_INTEGRATION_CLI environment variables.

If all you want is to specify a test filter to run, you can set the
`TEST_FILTER` environment variable. This ends up getting passed directly to `go
test -run` (or `go test -check-f`, depending on the test suite). It will also
automatically set the other above mentioned environment variables accordingly.

### Go Version

You can change a version of golang used for building stuff that is being tested
by setting `GO_VERSION` variable, for example:

```bash
make GO_VERSION=1.24.6 test
```
