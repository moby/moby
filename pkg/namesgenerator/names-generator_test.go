package namesgenerator

import (
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestNameFormat(c *check.C) {
	name := GetRandomName(0)
	if !strings.Contains(name, "_") {
		c.Fatalf("Generated name does not contain an underscore")
	}
	if strings.ContainsAny(name, "0123456789") {
		c.Fatalf("Generated name contains numbers!")
	}
}

func (s *DockerSuite) TestNameRetries(c *check.C) {
	name := GetRandomName(1)
	if !strings.Contains(name, "_") {
		c.Fatalf("Generated name does not contain an underscore")
	}
	if !strings.ContainsAny(name, "0123456789") {
		c.Fatalf("Generated name doesn't contain a number")
	}

}
