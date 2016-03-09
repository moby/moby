package main

import (
	"fmt"
	"net/http"

	"github.com/go-check/check"
)

// TestUserAgentPassThroughOnPull verifies that when an image is pulled from
// a registry, the registry should see a User-Agent string of the form
// "[client UA] upstream-ua/end; [docker engine UA]"
func (s *DockerRegistrySuite) TestUserAgentPassThroughOnPull(c *check.C) {
	reg, err := newTestRegistry(c)
	c.Assert(err, check.IsNil)
	expectUpstreamUA := false

	reg.registerHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		var ua string
		for k, v := range r.Header {
			if k == "User-Agent" {
				ua = v[0]
			}
		}
		c.Assert(ua, check.Not(check.Equals), "", check.Commentf("No User-Agent found in request"))
		if r.URL.Path == "/v2/busybox/manifests/latest" {
			if expectUpstreamUA {
				c.Assert(ua, check.Matches, ".+upstream\\-ua\\/end;.+", check.Commentf("Seperator token 'upstream-ua/end;' not found"))
			}
		}
	})

	repoName := fmt.Sprintf("%s/busybox", reg.hostport)
	err = s.d.Start("--insecure-registry", reg.hostport, "--disable-legacy-registry=true")
	c.Assert(err, check.IsNil)

	dockerfileName, cleanup, err := makefile(fmt.Sprintf("FROM %s/busybox", reg.hostport))
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test dockerfile"))
	defer cleanup()

	s.d.Cmd("build", "--file", dockerfileName, ".")

	s.d.Cmd("run", repoName)
	s.d.Cmd("login", "-u", "richard", "-p", "testtest", "-e", "testuser@testdomain.com", reg.hostport)
	s.d.Cmd("tag", "busybox", repoName)
	s.d.Cmd("push", repoName)

	expectUpstreamUA = true
	s.d.Cmd("pull", repoName)
}
