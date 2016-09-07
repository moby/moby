package useragent

import (
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestVersionInfo(c *check.C) {
	vi := VersionInfo{"foo", "bar"}
	if !vi.isValid() {
		c.Fatalf("VersionInfo should be valid")
	}
	vi = VersionInfo{"", "bar"}
	if vi.isValid() {
		c.Fatalf("Expected VersionInfo to be invalid")
	}
	vi = VersionInfo{"foo", ""}
	if vi.isValid() {
		c.Fatalf("Expected VersionInfo to be invalid")
	}
}

func (s *DockerSuite) TestAppendVersions(c *check.C) {
	vis := []VersionInfo{
		{"foo", "1.0"},
		{"bar", "0.1"},
		{"pi", "3.1.4"},
	}
	v := AppendVersions("base", vis...)
	expect := "base foo/1.0 bar/0.1 pi/3.1.4"
	if v != expect {
		c.Fatalf("expected %q, got %q", expect, v)
	}
}
