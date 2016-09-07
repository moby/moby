package sysinfo

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestIsCpusetListAvailable(c *check.C) {
	cases := []struct {
		provided  string
		available string
		res       bool
		err       bool
	}{
		{"1", "0-4", true, false},
		{"01,3", "0-4", true, false},
		{"", "0-7", true, false},
		{"1--42", "0-7", false, true},
		{"1-42", "00-1,8,,9", false, true},
		{"1,41-42", "43,45", false, false},
		{"0-3", "", false, false},
	}
	for _, ca := range cases {
		r, err := isCpusetListAvailable(ca.provided, ca.available)
		if (ca.err && err == nil) && r != ca.res {
			c.Fatalf("Expected pair: %v, %v for %s, %s. Got %v, %v instead", ca.res, ca.err, ca.provided, ca.available, (ca.err && err == nil), r)
		}
	}
}
