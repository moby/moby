/*Package golden provides tools for comparing large mutli-line strings.

Golden files are files in the ./testdata/ subdirectory of the package under test.
Golden files can be automatically updated to match new values by running
`go test pkgname -test.update-golden`. To ensure the update is correct
compare the diff of the old expected value to the new expected value.
*/
package golden // import "gotest.tools/v3/golden"

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/internal/format"
)

var flagUpdate = flag.Bool("test.update-golden", false, "update golden file")

type helperT interface {
	Helper()
}

// NormalizeCRLFToLF enables end-of-line normalization for actual values passed
// to Assert and String, as well as the values saved to golden files with
// -test.update-golden.
//
// Defaults to true. If you use the core.autocrlf=true git setting on windows
// you will need to set this to false.
//
// The value may be set to false by setting GOTESTTOOLS_GOLDEN_NormalizeCRLFToLF=false
// in the environment before running tests.
//
// The default value may change in a future major release.
var NormalizeCRLFToLF = os.Getenv("GOTESTTOOLS_GOLDEN_NormalizeCRLFToLF") != "false"

// FlagUpdate returns true when the -test.update-golden flag has been set.
func FlagUpdate() bool {
	return *flagUpdate
}

// Open opens the file in ./testdata
func Open(t assert.TestingT, filename string) *os.File {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	f, err := os.Open(Path(filename))
	assert.NilError(t, err)
	return f
}

// Get returns the contents of the file in ./testdata
func Get(t assert.TestingT, filename string) []byte {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	expected, err := ioutil.ReadFile(Path(filename))
	assert.NilError(t, err)
	return expected
}

// Path returns the full path to a file in ./testdata
func Path(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join("testdata", filename)
}

func removeCarriageReturn(in []byte) []byte {
	if !NormalizeCRLFToLF {
		return in
	}
	return bytes.Replace(in, []byte("\r\n"), []byte("\n"), -1)
}

// Assert compares actual to the expected value in the golden file.
//
// Running `go test pkgname -test.update-golden` will write the value of actual
// to the golden file.
//
// This is equivalent to assert.Assert(t, String(actual, filename))
func Assert(t assert.TestingT, actual string, filename string, msgAndArgs ...interface{}) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert.Assert(t, String(actual, filename), msgAndArgs...)
}

// String compares actual to the contents of filename and returns success
// if the strings are equal.
//
// Running `go test pkgname -test.update-golden` will write the value of actual
// to the golden file.
//
// Any \r\n substrings in actual are converted to a single \n character
// before comparing it to the expected string. When updating the golden file the
// normalized version will be written to the file. This allows Windows to use
// the same golden files as other operating systems.
func String(actual string, filename string) cmp.Comparison {
	return func() cmp.Result {
		actualBytes := removeCarriageReturn([]byte(actual))
		result, expected := compare(actualBytes, filename)
		if result != nil {
			return result
		}
		diff := format.UnifiedDiff(format.DiffConfig{
			A:    string(expected),
			B:    string(actualBytes),
			From: "expected",
			To:   "actual",
		})
		return cmp.ResultFailure("\n" + diff + failurePostamble(filename))
	}
}

func failurePostamble(filename string) string {
	return fmt.Sprintf(`

You can run 'go test . -test.update-golden' to automatically update %s to the new expected value.'
`, Path(filename))
}

// AssertBytes compares actual to the expected value in the golden.
//
// Running `go test pkgname -test.update-golden` will write the value of actual
// to the golden file.
//
// This is equivalent to assert.Assert(t, Bytes(actual, filename))
func AssertBytes(
	t assert.TestingT,
	actual []byte,
	filename string,
	msgAndArgs ...interface{},
) {
	if ht, ok := t.(helperT); ok {
		ht.Helper()
	}
	assert.Assert(t, Bytes(actual, filename), msgAndArgs...)
}

// Bytes compares actual to the contents of filename and returns success
// if the bytes are equal.
//
// Running `go test pkgname -test.update-golden` will write the value of actual
// to the golden file.
func Bytes(actual []byte, filename string) cmp.Comparison {
	return func() cmp.Result {
		result, expected := compare(actual, filename)
		if result != nil {
			return result
		}
		msg := fmt.Sprintf("%v (actual) != %v (expected)", actual, expected)
		return cmp.ResultFailure(msg + failurePostamble(filename))
	}
}

func compare(actual []byte, filename string) (cmp.Result, []byte) {
	if err := update(filename, actual); err != nil {
		return cmp.ResultFromError(err), nil
	}
	expected, err := ioutil.ReadFile(Path(filename))
	if err != nil {
		return cmp.ResultFromError(err), nil
	}
	if bytes.Equal(expected, actual) {
		return cmp.ResultSuccess, nil
	}
	return nil, expected
}

func update(filename string, actual []byte) error {
	if !*flagUpdate {
		return nil
	}
	if dir := filepath.Dir(Path(filename)); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return ioutil.WriteFile(Path(filename), actual, 0644)
}
