package main

import (
	"encoding/json"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestGetVersion(c *check.C) {
	_, body, err := sockRequest("GET", "/version", nil)
	if err != nil {
		c.Fatal(err)
	}
	var v types.Version
	if err := json.Unmarshal(body, &v); err != nil {
		c.Fatal(err)
	}

	if v.Version != dockerversion.VERSION {
		c.Fatal("Version mismatch")
	}
}
