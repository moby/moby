package daemon

import (
	"github.com/docker/docker/volume"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestParseVolumesFrom(c *check.C) {
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

	for _, ca := range cases {
		id, mode, err := volume.ParseVolumesFrom(ca.spec)
		if ca.fail {
			if err == nil {
				c.Fatalf("Expected error, was nil, for spec %s\n", ca.spec)
			}
			continue
		}

		if id != ca.expID {
			c.Fatalf("Expected id %s, was %s, for spec %s\n", ca.expID, id, ca.spec)
		}
		if mode != ca.expMode {
			c.Fatalf("Expected mode %s, was %s for spec %s\n", ca.expMode, mode, ca.spec)
		}
	}
}
