package system

import "github.com/go-check/check"

func (s *DockerSuite) TestHasWin32KSupport(c *check.C) {
	sp := HasWin32KSupport() // make sure this doesn't panic

	c.Logf("win32k: %v", sp) // will be different on different platforms -- informative only
}
