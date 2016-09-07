package httputils

import "github.com/go-check/check"

func (s *DockerSuite) TestDetectContentType(c *check.C) {
	input := []byte("That is just a plain text")

	if contentType, _, err := DetectContentType(input); err != nil || contentType != "text/plain" {
		c.Errorf("TestDetectContentType failed")
	}
}
