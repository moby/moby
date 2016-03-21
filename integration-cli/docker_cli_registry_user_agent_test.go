package main

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-check/check"
)

// unescapeBackslashSemicolonParens unescapes \;()
func unescapeBackslashSemicolonParens(s string) string {
	re := regexp.MustCompile("\\\\;")
	ret := re.ReplaceAll([]byte(s), []byte(";"))

	re = regexp.MustCompile("\\\\\\(")
	ret = re.ReplaceAll([]byte(ret), []byte("("))

	re = regexp.MustCompile("\\\\\\)")
	ret = re.ReplaceAll([]byte(ret), []byte(")"))

	re = regexp.MustCompile("\\\\\\\\")
	ret = re.ReplaceAll([]byte(ret), []byte("\\"))

	return string(ret)
}

func regexpCheckUA(c *check.C, ua string) {
	re := regexp.MustCompile("(?P<dockerUA>.+) UpstreamClient(?P<upstreamUA>.+)")
	substrArr := re.FindStringSubmatch(ua)

	c.Assert(substrArr, check.HasLen, 3, check.Commentf("Expected 'UpstreamClient()' with upstream client UA"))
	dockerUA := substrArr[1]
	upstreamUAEscaped := substrArr[2]

	// check dockerUA looks correct
	reDockerUA := regexp.MustCompile("^docker/[0-9A-Za-z+]")
	bMatchDockerUA := reDockerUA.MatchString(dockerUA)
	c.Assert(bMatchDockerUA, check.Equals, true, check.Commentf("Docker Engine User-Agent malformed"))

	// check upstreamUA looks correct
	// Expecting something like:  Docker-Client/1.11.0-dev (linux)
	upstreamUA := unescapeBackslashSemicolonParens(upstreamUAEscaped)
	reUpstreamUA := regexp.MustCompile("^\\(Docker-Client/[0-9A-Za-z+]")
	bMatchUpstreamUA := reUpstreamUA.MatchString(upstreamUA)
	c.Assert(bMatchUpstreamUA, check.Equals, true, check.Commentf("(Upstream) Docker Client User-Agent malformed"))
}

// TestUserAgentPassThroughOnPull verifies that when an image is pulled from
// a registry, the registry should see a User-Agent string of the form
//   [docker engine UA] UptreamClientSTREAM-CLIENT([client UA])
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
				regexpCheckUA(c, ua)
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
