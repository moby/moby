package main

import (
	"net/http"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiCreateWithNotExistImage(c *check.C) {
	name := "test"
	config := map[string]interface{}{
		"Image":   "test456:v1",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, resp, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	expected := "No such image: test456:v1"
	if !strings.Contains(string(resp), expected) {
		c.Fatalf("expected: %s, got: %s", expected, string(resp))
	}

	config2 := map[string]interface{}{
		"Image":   "test456",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, resp, err = sockRequest("POST", "/containers/create?name="+name, config2)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	expected = "No such image: test456:latest"
	if !strings.Contains(string(resp), expected) {
		c.Fatalf("expected: %s, got: %s", expected, string(resp))
	}

}
