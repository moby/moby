package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	volumemounts "github.com/docker/docker/volume/mounts"
)

func TestParseVolumesFrom(t *testing.T) {
	cases := []struct {
		spec    string
		expID   string
		expMode string
		fail    bool
	}{
		{"", "", "", true},
		{"foobar", "foobar", "rw", false},
		{"foobar:rw", "foobar", "rw", false},
		{"foobar:ro", "foobar", "ro", false},
		{"foobar:baz", "", "", true},
	}

	parser := volumemounts.NewParser()

	for _, c := range cases {
		id, mode, err := parser.ParseVolumesFrom(c.spec)
		if c.fail {
			if err == nil {
				t.Fatalf("Expected error, was nil, for spec %s\n", c.spec)
			}
			continue
		}

		if id != c.expID {
			t.Fatalf("Expected id %s, was %s, for spec %s\n", c.expID, id, c.spec)
		}
		if mode != c.expMode {
			t.Fatalf("Expected mode %s, was %s for spec %s\n", c.expMode, mode, c.spec)
		}
	}
}
