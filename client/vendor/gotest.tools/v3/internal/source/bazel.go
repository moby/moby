package source

import (
	"fmt"
	"os"
	"path/filepath"
)

// These Bazel env vars are documented here:
// https://bazel.build/reference/test-encyclopedia

// Signifies test executable is being driven by `bazel test`.
//
// Due to Bazel's compilation and sandboxing strategy,
// some care is required to handle resolving the original *.go source file.
var inBazelTest = os.Getenv("BAZEL_TEST") == "1"

// The name of the target being tested (ex: //some_package:some_package_test)
var bazelTestTarget = os.Getenv("TEST_TARGET")

// Absolute path to the base of the runfiles tree
var bazelTestSrcdir = os.Getenv("TEST_SRCDIR")

// The local repository's workspace name (ex: __main__)
var bazelTestWorkspace = os.Getenv("TEST_WORKSPACE")

func bazelSourcePath(filename string) (string, error) {
	// Use the env vars to resolve the test source files,
	// which must be provided as test data in the respective go_test target.
	filename = filepath.Join(bazelTestSrcdir, bazelTestWorkspace, filename)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return "", fmt.Errorf(bazelMissingSourceMsg, filename, bazelTestTarget)
	}
	return filename, nil
}

var bazelMissingSourceMsg = `
the test source file does not exist: %s
It appears that you are running this test under Bazel (target: %s).
Check that your test source files are added as test data in your go_test targets.

Example:
    go_test(
        name = "your_package_test",
        srcs = ["your_test.go"],
        deps = ["@tools_gotest_v3//assert"],
        data = glob(["*_test.go"])
    )"
`
