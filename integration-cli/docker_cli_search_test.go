package main

import (
	"strings"

	"github.com/go-check/check"
)

// search for repos named  "registry" on the central registry
func (s *DockerSuite) TestSearchOnCentralRegistry(c *check.C) {
	testRequires(c, Network)

	out := dockerCmd(c, "search", "busybox")

	if !strings.Contains(out, "Busybox base image.") {
		c.Fatal("couldn't find any repository named (or containing) 'Busybox base image.'")
	}
}

func (s *DockerSuite) TestSearchStarsOptionWithWrongParameter(c *check.C) {
	out, exitCode, err := dockerCmdWithError("search", "--stars=a", "busybox")
	if err == nil || exitCode == 0 {
		c.Fatalf("Should not get right information: %s, %v", out, err)
	}

	if !strings.Contains(out, "invalid value") {
		c.Fatal("couldn't find the invalid value warning")
	}

	out, exitCode, err = dockerCmdWithError("search", "-s=-1", "busybox")
	if err == nil || exitCode == 0 {
		c.Fatalf("Should not get right information: %s, %v", out, err)
	}

	if !strings.Contains(out, "invalid value") {
		c.Fatal("couldn't find the invalid value warning")
	}
}

func (s *DockerSuite) TestSearchCmdOptions(c *check.C) {
	testRequires(c, Network)

	out := dockerCmd(c, "search", "--help")

	if !strings.Contains(out, "Usage:\tdocker search [OPTIONS] TERM") {
		c.Fatalf("failed to show docker search usage: %s", out)
	}

	outSearchCmd := dockerCmd(c, "search", "busybox")

	outSearchCmdNotrunc := dockerCmd(c, "search", "--no-trunc=true", "busybox")

	if len(outSearchCmd) > len(outSearchCmdNotrunc) {
		c.Fatalf("The no-trunc option can't take effect.")
	}

	outSearchCmdautomated := dockerCmd(c, "search", "--automated=true", "busybox") //The busybox is a busybox base image, not an AUTOMATED image.

	outSearchCmdautomatedSlice := strings.Split(outSearchCmdautomated, "\n")
	for i := range outSearchCmdautomatedSlice {
		if strings.HasPrefix(outSearchCmdautomatedSlice[i], "busybox ") {
			c.Fatalf("The busybox is not an AUTOMATED image: %s", out)
		}
	}

	outSearchCmdStars := dockerCmd(c, "search", "-s=2", "busybox")

	if strings.Count(outSearchCmdStars, "[OK]") > strings.Count(outSearchCmd, "[OK]") {
		c.Fatalf("The quantity of images with stars should be less than that of all images: %s", outSearchCmdStars)
	}

	dockerCmd(c, "search", "--stars=2", "--automated=true", "--no-trunc=true", "busybox")
}
