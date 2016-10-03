package main

import (
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestAPICreateWithNotExistImage(c *check.C) {
	name := "test"
	config := map[string]interface{}{
		"Image":   "test456:v1",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, body, err := sockRequest("POST", "/containers/create?name="+name, config)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	expected := "No such image: test456:v1"
	c.Assert(getErrorMessage(c, body), checker.Contains, expected)

	config2 := map[string]interface{}{
		"Image":   "test456",
		"Volumes": map[string]struct{}{"/tmp": {}},
	}

	status, body, err = sockRequest("POST", "/containers/create?name="+name, config2)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	expected = "No such image: test456:latest"
	c.Assert(getErrorMessage(c, body), checker.Equals, expected)

	config3 := map[string]interface{}{
		"Image": "sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
	}

	status, body, err = sockRequest("POST", "/containers/create?name="+name, config3)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNotFound)
	expected = "No such image: sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa"
	c.Assert(getErrorMessage(c, body), checker.Equals, expected)

}
