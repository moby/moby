package main

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/docker/docker/integration-cli/registry"
	"github.com/go-check/check"
)

// unescapeBackslashSemicolonParens unescapes \;()
func unescapeBackslashSemicolonParens(s string) string {
	re := regexp.MustCompile(`\\;`)
	ret := re.ReplaceAll([]byte(s), []byte(";"))

	re = regexp.MustCompile(`\\\(`)
	ret = re.ReplaceAll([]byte(ret), []byte("("))

	re = regexp.MustCompile(`\\\)`)
	ret = re.ReplaceAll([]byte(ret), []byte(")"))

	re = regexp.MustCompile(`\\\\`)
	ret = re.ReplaceAll([]byte(ret), []byte(`\`))

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

func registerUserAgentHandler(reg *registry.Mock, result *string) {
	reg.RegisterHandler("/v2/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		var ua string
		for k, v := range r.Header {
			if k == "User-Agent" {
				ua = v[0]
			}
		}
		*result = ua
	})
}

// TestUserAgentPassThrough verifies that when an image is pulled from
// a registry, the registry should see a User-Agent string of the form
// [docker engine UA] UpstreamClientSTREAM-CLIENT([client UA])
func (s *DockerRegistrySuite) TestUserAgentPassThrough(c *check.C) {
	var (
		buildUA string
		pullUA  string
		pushUA  string
		loginUA string
	)

	buildReg, err := registry.NewMock(c)
	defer buildReg.Close()
	c.Assert(err, check.IsNil)
	registerUserAgentHandler(buildReg, &buildUA)
	buildRepoName := fmt.Sprintf("%s/busybox", buildReg.URL())

	pullReg, err := registry.NewMock(c)
	defer pullReg.Close()
	c.Assert(err, check.IsNil)
	registerUserAgentHandler(pullReg, &pullUA)
	pullRepoName := fmt.Sprintf("%s/busybox", pullReg.URL())

	pushReg, err := registry.NewMock(c)
	defer pushReg.Close()
	c.Assert(err, check.IsNil)
	registerUserAgentHandler(pushReg, &pushUA)
	pushRepoName := fmt.Sprintf("%s/busybox", pushReg.URL())

	loginReg, err := registry.NewMock(c)
	defer loginReg.Close()
	c.Assert(err, check.IsNil)
	registerUserAgentHandler(loginReg, &loginUA)

	s.d.Start(c,
		"--insecure-registry", buildReg.URL(),
		"--insecure-registry", pullReg.URL(),
		"--insecure-registry", pushReg.URL(),
		"--insecure-registry", loginReg.URL(),
		"--disable-legacy-registry=true")

	dockerfileName, cleanup1, err := makefile(fmt.Sprintf("FROM %s", buildRepoName))
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test dockerfile"))
	defer cleanup1()
	s.d.Cmd("build", "--file", dockerfileName, ".")
	regexpCheckUA(c, buildUA)

	s.d.Cmd("login", "-u", "richard", "-p", "testtest", "-e", "testuser@testdomain.com", loginReg.URL())
	regexpCheckUA(c, loginUA)

	s.d.Cmd("pull", pullRepoName)
	regexpCheckUA(c, pullUA)

	dockerfileName, cleanup2, err := makefile(`FROM scratch
	ENV foo bar`)
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test dockerfile"))
	defer cleanup2()
	s.d.Cmd("build", "-t", pushRepoName, "--file", dockerfileName, ".")

	s.d.Cmd("push", pushRepoName)
	regexpCheckUA(c, pushUA)
}
