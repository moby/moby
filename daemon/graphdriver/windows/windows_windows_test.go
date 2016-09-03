package windows

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestAddAceToSddlDacl(c *check.C) {
	cases := [][3]string{
		{"D:", "(A;;;)", "D:(A;;;)"},
		{"D:(A;;;)", "(A;;;)", "D:(A;;;)"},
		{"O:D:(A;;;stuff)", "(A;;;new)", "O:D:(A;;;new)(A;;;stuff)"},
		{"O:D:(D;;;no)(A;;;stuff)", "(A;;;new)", "O:D:(D;;;no)(A;;;new)(A;;;stuff)"},
	}

	for _, ca := range cases {
		if newSddl, worked := addAceToSddlDacl(ca[0], ca[1]); !worked || newSddl != ca[2] {
			c.Errorf("%s + %s == %s, expected %s (%v)", ca[0], ca[1], newSddl, ca[2], worked)
		}
	}
}
