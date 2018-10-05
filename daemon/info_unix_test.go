// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestParseInitVersion(t *testing.T) {
	tests := []struct {
		output  string
		version string
		commit  string
		invalid bool
	}{
		{
			output:  "tini version 0.13.0 - git.949e6fa",
			version: "0.13.0",
			commit:  "949e6fa",
		}, {
			output:  "tini version 0.13.0\n",
			version: "0.13.0",
		}, {
			output:  "tini version 0.13.2",
			version: "0.13.2",
		}, {
			output:  "tini version0.13.2",
			invalid: true,
		}, {
			output:  "",
			invalid: true,
		}, {
			output:  "hello world",
			invalid: true,
		},
	}

	for _, test := range tests {
		version, commit, err := parseInitVersion(string(test.output))
		if test.invalid {
			assert.Check(t, is.ErrorContains(err, ""))
		} else {
			assert.Check(t, err)
		}
		assert.Equal(t, test.version, version)
		assert.Equal(t, test.commit, commit)
	}
}

func TestParseRuncVersion(t *testing.T) {
	tests := []struct {
		output  string
		version string
		commit  string
		invalid bool
	}{
		{
			output: `
runc version 1.0.0-rc5+dev
commit: 69663f0bd4b60df09991c08812a60108003fa340
spec: 1.0.0
`,
			version: "1.0.0-rc5+dev",
			commit:  "69663f0bd4b60df09991c08812a60108003fa340",
		},
		{
			output: `
runc version 1.0.0-rc5+dev
spec: 1.0.0
`,
			version: "1.0.0-rc5+dev",
		},
		{
			output: `
commit: 69663f0bd4b60df09991c08812a60108003fa340
spec: 1.0.0
`,
			commit: "69663f0bd4b60df09991c08812a60108003fa340",
		},
		{
			output:  "",
			invalid: true,
		},
		{
			output:  "hello world",
			invalid: true,
		},
	}

	for _, test := range tests {
		version, commit, err := parseRuncVersion(string(test.output))
		if test.invalid {
			assert.Check(t, is.ErrorContains(err, ""))
		} else {
			assert.Check(t, err)
		}
		assert.Equal(t, test.version, version)
		assert.Equal(t, test.commit, commit)
	}
}
