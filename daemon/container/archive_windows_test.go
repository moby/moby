//go:build windows

package container

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestAssertsDirectory(t *testing.T) {
	sep := string(filepath.Separator)
	for _, tc := range []struct {
		path     string
		expected bool
	}{
		{path: "", expected: false},
		{path: `\foo`, expected: false},
		{path: `\foo\bar`, expected: false},
		{path: `\foo\`, expected: true},
		{path: sep, expected: true},
		{path: `\foo\.`, expected: true},
		{path: `.`, expected: true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Check(t, is.Equal(assertsDirectory(tc.path), tc.expected))
		})
	}
}
