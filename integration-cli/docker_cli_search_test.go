package main

import (
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// search for repos named  "registry" on the central registry
func (s *DockerSuite) TestSearchOnCentralRegistry(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	out, _ := dockerCmd(c, "search", "busybox")
	c.Assert(out, checker.Contains, "Busybox base image.", check.Commentf("couldn't find any repository named (or containing) 'Busybox base image.'"))
}

func (s *DockerSuite) TestSearchStarsOptionWithWrongParameter(c *check.C) {
	out, _, err := dockerCmdWithError("search", "--stars=a", "busybox")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "invalid value", check.Commentf("couldn't find the invalid value warning"))

	out, _, err = dockerCmdWithError("search", "-s=-1", "busybox")
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "invalid value", check.Commentf("couldn't find the invalid value warning"))
}

func (s *DockerSuite) TestSearchCmdOptions(c *check.C) {
	testRequires(c, Network)

	out, _ := dockerCmd(c, "search", "--help")
	c.Assert(out, checker.Contains, "Usage:\tdocker search [OPTIONS] TERM")

	outSearchCmd, _ := dockerCmd(c, "search", "busybox")
	outSearchCmdNotrunc, _ := dockerCmd(c, "search", "--no-trunc=true", "busybox")
	c.Assert(len(outSearchCmd) > len(outSearchCmdNotrunc), check.Equals, false, check.Commentf("The no-trunc option can't take effect."))

	outSearchCmdautomated, _ := dockerCmd(c, "search", "--automated=true", "busybox") //The busybox is a busybox base image, not an AUTOMATED image.
	outSearchCmdautomatedSlice := strings.Split(outSearchCmdautomated, "\n")
	for i := range outSearchCmdautomatedSlice {
		c.Assert(strings.HasPrefix(outSearchCmdautomatedSlice[i], "busybox "), check.Equals, false, check.Commentf("The busybox is not an AUTOMATED image: %s", out))
	}

	outSearchCmdStars, _ := dockerCmd(c, "search", "-s=2", "busybox")
	c.Assert(strings.Count(outSearchCmdStars, "[OK]") > strings.Count(outSearchCmd, "[OK]"), check.Equals, false, check.Commentf("The quantity of images with stars should be less than that of all images: %s", outSearchCmdStars))

	dockerCmd(c, "search", "--stars=2", "--automated=true", "--no-trunc=true", "busybox")
}

// search for repos which start with "ubuntu-" on the central registry
func (s *DockerSuite) TestSearchOnCentralRegistryWithDash(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	dockerCmd(c, "search", "ubuntu-")
}
