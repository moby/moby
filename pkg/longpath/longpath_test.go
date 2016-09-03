package longpath

import (
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestStandardLongPath(c *check.C) {
	sp := `C:\simple\path`
	longC := AddPrefix(sp)
	if !strings.EqualFold(longC, `\\?\C:\simple\path`) {
		c.Errorf("Wrong long path returned. Original = %s ; Long = %s", sp, longC)
	}
}

func (s *DockerSuite) TestUNCLongPath(c *check.C) {
	sp := `\\server\share\path`
	longC := AddPrefix(sp)
	if !strings.EqualFold(longC, `\\?\UNC\server\share\path`) {
		c.Errorf("Wrong UNC long path returned. Original = %s ; Long = %s", sp, longC)
	}
}
