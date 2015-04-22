package main

import (
	"net/http"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestInfoApi(c *check.C) {
	endpoint := "/info"

	statusCode, body, err := sockRequest("GET", endpoint, nil)
	if err != nil || statusCode != http.StatusOK {
		c.Fatalf("Expected %d from info request, got %d", http.StatusOK, statusCode)
	}

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"Images",
		"ExecutionDriver",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"MemTotal",
		"KernelVersion",
		"Driver"}

	out := string(body)
	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			c.Errorf("couldn't find string %v in output", linePrefix)
		}
	}
}
