//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestContainerTopValidatePSArgs(t *testing.T) {
	tests := map[string]bool{
		"ae -o uid=PID":             true,
		"ae -o \"uid= PID\"":        true,  // ascii space (0x20)
		"ae -o \"uid= PID\"":        false, // unicode space (U+2003, 0xe2 0x80 0x83)
		"ae o uid=PID":              true,
		"aeo uid=PID":               true,
		"ae -O uid=PID":             true,
		"ae -o pid=PID2 -o uid=PID": true,
		"ae -o pid=PID":             false,
		"ae -o pid=PID -o uid=PIDX": true, // FIXME: we do not need to prohibit this
		"aeo pid=PID":               false,
		"ae":                        false,
		"":                          false,
	}
	for psArgs, errExpected := range tests {
		t.Run(psArgs, func(t *testing.T) {
			err := validatePSArgs(psArgs)
			if errExpected {
				assert.ErrorContains(t, err, "", "psArgs: %q", psArgs)
			} else {
				assert.NilError(t, err, "psArgs: %q", psArgs)
			}
		})
	}
}

func TestContainerTopParsePSOutput(t *testing.T) {
	tests := []struct {
		output      []byte
		pids        []uint32
		errExpected bool
	}{
		{
			output: []byte(`  PID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`),
			pids:        []uint32{42, 43},
			errExpected: false,
		},
		{
			output: []byte(`  UID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`),
			pids:        []uint32{42, 43},
			errExpected: true,
		},
		// unicode space (U+2003, 0xe2 0x80 0x83)
		{
			output: []byte(` PID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`),
			pids:        []uint32{42, 43},
			errExpected: true,
		},
		// the first space is U+2003, the second one is ascii.
		{
			output: []byte(` PID COMMAND
   42 foo
   43 bar
  100 baz
`),
			pids:        []uint32{42, 43},
			errExpected: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.output), func(t *testing.T) {
			_, err := parsePSOutput(tc.output, tc.pids)
			if tc.errExpected && err == nil {
				t.Fatalf("expected error, got %v (%q)", err, string(tc.output))
			}
			if !tc.errExpected && err != nil {
				t.Fatalf("expected nil, got %v (%q)", err, string(tc.output))
			}
		})
	}
}
