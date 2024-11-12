//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
			output:  "tini version 0.13.2 - ",
			version: "0.13.2",
		}, {
			output: " - git.949e6fa",
			commit: "949e6fa",
		}, {
			output:  "tini version0.13.2",
			invalid: true,
		}, {
			output:  "version 0.13.0",
			invalid: true,
		}, {
			output:  "",
			invalid: true,
		}, {
			output:  " - ",
			invalid: true,
		}, {
			output:  "hello world",
			invalid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.output, func(t *testing.T) {
			version, commit, err := parseInitVersion(tc.output)
			if tc.invalid {
				assert.Check(t, is.ErrorContains(err, ""))
			} else {
				assert.Check(t, err)
			}
			assert.Equal(t, tc.version, version)
			assert.Equal(t, tc.commit, commit)
		})
	}
}

func TestParseRuntimeVersion(t *testing.T) {
	tests := []struct {
		output  string
		runtime string
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
			runtime: "runc",
			version: "1.0.0-rc5+dev",
			commit:  "69663f0bd4b60df09991c08812a60108003fa340",
		},
		{
			output: `
runc version 1.0.0-rc5+dev
spec: 1.0.0
`,
			runtime: "runc",
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
			output: `
crun version 0.7
spec: 1.0.0
+SYSTEMD +SELINUX +CAP +SECCOMP +EBPF +YAJL
`,
			runtime: "crun",
			version: "0.7",
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

	for _, tc := range tests {
		runtime, version, commit, err := parseRuntimeVersion(tc.output)
		if tc.invalid {
			assert.Check(t, is.ErrorContains(err, ""))
		} else {
			assert.Check(t, err)
		}
		assert.Equal(t, tc.runtime, runtime)
		assert.Equal(t, tc.version, version)
		assert.Equal(t, tc.commit, commit)
	}
}
