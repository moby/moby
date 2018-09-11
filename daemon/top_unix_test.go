//+build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestContainerTopParsePSOutput(t *testing.T) {
	tests := []struct {
		output      []byte
		pids        []uint32
		errExpected bool
	}{
		{[]byte(`  PID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`), []uint32{42, 43}, false},
		{[]byte(`  UID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`), []uint32{42, 43}, true},
		// unicode space (U+2003, 0xe2 0x80 0x83)
		{[]byte(` PID COMMAND
   42 foo
   43 bar
		- -
  100 baz
`), []uint32{42, 43}, true},
		// the first space is U+2003, the second one is ascii.
		{[]byte(` PID COMMAND
   42 foo
   43 bar
  100 baz
`), []uint32{42, 43}, true},
	}

	for _, f := range tests {
		_, err := parsePSOutput(f.output, f.pids, false)
		t.Logf("tested %q, got err=%v", string(f.output), err)
		if f.errExpected && err == nil {
			t.Fatalf("expected error, got %v (%q)", err, string(f.output))
		}
		if !f.errExpected && err != nil {
			t.Fatalf("expected nil, got %v (%q)", err, string(f.output))
		}
	}
}

func TestContainerTopCustomFields(t *testing.T) {
	cases := []struct {
		opts   string
		custom bool
	}{
		{"", false},
		{"-C sleep", false},
		{"-C sleep ocmd", true},
		{"-Csleep ocmd", true},
		{"-Csto", false},
		{"-o cmd", true},
		{"-o=cmd", true},
		{"-ocmd", true},
		{"eocmd", true},
		{"--format cmd", true},
		{"--format=cmd", true},
		{"-A eocmd", true},
		{"-U eocmd", false},
		{"efUocmd", false},
		{"aux --sort=comm", false},
		{"-C sleep eocmd,uid", true},
	}
	for _, c := range cases {
		assert.Check(t, is.Equal(customFields(fieldsASCII(c.opts)), c.custom), c.opts)
	}
}
