package main

import (
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInfoApi(c *check.C) {
	endpoint := "/info"

	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"Images",
		"ExecutionDriver",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"OSType",
		"Architecture",
		"MemTotal",
		"KernelVersion",
		"Driver",
		"ServerVersion"}

	out := string(body)
	for _, linePrefix := range stringsToCheck {
		c.Assert(out, checker.Contains, linePrefix)
	}
}
