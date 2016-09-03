package homedir

import (
	"path/filepath"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestGet(c *check.C) {
	home := Get()
	if home == "" {
		c.Fatal("returned home directory is empty")
	}

	if !filepath.IsAbs(home) {
		c.Fatalf("returned path is not absolute: %s", home)
	}
}

func (s *DockerSuite) TestGetShortcutString(c *check.C) {
	shortcut := GetShortcutString()
	if shortcut == "" {
		c.Fatal("returned shortcut string is empty")
	}
}
