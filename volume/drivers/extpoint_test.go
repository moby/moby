package volumedrivers

import (
	"testing"

	"github.com/docker/docker/volume/testutils"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestGetDriver(c *check.C) {
	_, err := GetDriver("missing")
	if err == nil {
		c.Fatal("Expected error, was nil")
	}

	Register(volumetestutils.NewFakeDriver("fake"), "fake")
	d, err := GetDriver("fake")
	if err != nil {
		c.Fatal(err)
	}
	if d.Name() != "fake" {
		c.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}
