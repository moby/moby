package main

import (
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

type DockerCLISearchSuite struct {
	ds *DockerSuite
}

func (s *DockerCLISearchSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLISearchSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// search for repos named  "registry" on the central registry
func (s *DockerCLISearchSuite) TestSearchOnCentralRegistry(c *testing.T) {
	out, _ := dockerCmd(c, "search", "busybox")
	assert.Assert(c, strings.Contains(out, "Busybox base image."), "couldn't find any repository named (or containing) 'Busybox base image.'")
}

func (s *DockerCLISearchSuite) TestSearchStarsOptionWithWrongParameter(c *testing.T) {
	out, _, err := dockerCmdWithError("search", "--filter", "stars=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "stars=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "is-automated=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "is-official=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid filter"), "couldn't find the invalid filter warning")
}

func (s *DockerCLISearchSuite) TestSearchCmdOptions(c *testing.T) {
	outSearchCmd, _ := dockerCmd(c, "search", "busybox")
	assert.Assert(c, strings.Count(outSearchCmd, "\n") > 3, outSearchCmd)

	outSearchCmdautomated, _ := dockerCmd(c, "search", "--filter", "is-automated=true", "busybox") // The busybox is a busybox base image, not an AUTOMATED image.
	outSearchCmdautomatedSlice := strings.Split(outSearchCmdautomated, "\n")
	for i := range outSearchCmdautomatedSlice {
		assert.Assert(c, !strings.HasPrefix(outSearchCmdautomatedSlice[i], "busybox "), "The busybox is not an AUTOMATED image: %s", outSearchCmdautomated)
	}

	outSearchCmdNotOfficial, _ := dockerCmd(c, "search", "--filter", "is-official=false", "busybox") // The busybox is a busybox base image, official image.
	outSearchCmdNotOfficialSlice := strings.Split(outSearchCmdNotOfficial, "\n")
	for i := range outSearchCmdNotOfficialSlice {
		assert.Assert(c, !strings.HasPrefix(outSearchCmdNotOfficialSlice[i], "busybox "), "The busybox is not an OFFICIAL image: %s", outSearchCmdNotOfficial)
	}

	outSearchCmdOfficial, _ := dockerCmd(c, "search", "--filter", "is-official=true", "busybox") // The busybox is a busybox base image, official image.
	outSearchCmdOfficialSlice := strings.Split(outSearchCmdOfficial, "\n")
	assert.Equal(c, len(outSearchCmdOfficialSlice), 3) // 1 header, 1 line, 1 carriage return
	assert.Assert(c, strings.HasPrefix(outSearchCmdOfficialSlice[1], "busybox "), "The busybox is an OFFICIAL image: %s", outSearchCmdOfficial)

	outSearchCmdStars, _ := dockerCmd(c, "search", "--filter", "stars=10", "busybox")
	assert.Assert(c, strings.Count(outSearchCmdStars, "\n") <= strings.Count(outSearchCmd, "\n"), "Number of images with 10+ stars should be less than that of all images:\noutSearchCmdStars: %s\noutSearch: %s\n", outSearchCmdStars, outSearchCmd)

	dockerCmd(c, "search", "--filter", "is-automated=true", "--filter", "stars=2", "--no-trunc=true", "busybox")
}

// search for repos which start with "ubuntu-" on the central registry
func (s *DockerCLISearchSuite) TestSearchOnCentralRegistryWithDash(c *testing.T) {
	dockerCmd(c, "search", "ubuntu-")
}

// test case for #23055
func (s *DockerCLISearchSuite) TestSearchWithLimit(c *testing.T) {
	for _, limit := range []int{10, 50, 100} {
		out, _, err := dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
		assert.NilError(c, err)
		outSlice := strings.Split(out, "\n")
		assert.Equal(c, len(outSlice), limit+2) // 1 header, 1 carriage return
	}

	for _, limit := range []int{-1, 101} {
		_, _, err := dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
		assert.ErrorContains(c, err, "")
	}
}
