// +build linux

package seccomp

import (
	"io/ioutil"
	"testing"

	"github.com/docker/docker/oci"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestLoadProfile(c *check.C) {
	f, err := ioutil.ReadFile("fixtures/example.json")
	if err != nil {
		c.Fatal(err)
	}
	rs := oci.DefaultSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestLoadDefaultProfile(c *check.C) {
	f, err := ioutil.ReadFile("default.json")
	if err != nil {
		c.Fatal(err)
	}
	rs := oci.DefaultSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		c.Fatal(err)
	}
}
