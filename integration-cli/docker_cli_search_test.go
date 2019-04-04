package main

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
	"gotest.tools/assert"
)

// search for repos named  "registry" on the central registry
func (s *DockerSuite) TestSearchOnCentralRegistry(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	out, _ := dockerCmd(c, "search", "busybox")
	assert.Assert(c, strings.Contains(out, "Busybox base image."), "couldn't find any repository named (or containing) 'Busybox base image.'")
}

func (s *DockerSuite) TestSearchStarsOptionWithWrongParameter(c *check.C) {
	out, _, err := dockerCmdWithError("search", "--filter", "stars=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "Invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "stars=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "Invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "is-automated=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "Invalid filter"), "couldn't find the invalid filter warning")

	out, _, err = dockerCmdWithError("search", "-f", "is-official=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "Invalid filter"), "couldn't find the invalid filter warning")

	// -s --stars deprecated since Docker 1.13
	out, _, err = dockerCmdWithError("search", "--stars=a", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid syntax"), "couldn't find the invalid value warning")

	// -s --stars deprecated since Docker 1.13
	out, _, err = dockerCmdWithError("search", "-s=-1", "busybox")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "invalid syntax"), "couldn't find the invalid value warning")
}

func (s *DockerSuite) TestSearchCmdOptions(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	out, _ := dockerCmd(c, "search", "--help")
	assert.Assert(c, strings.Contains(out, "Usage:\tdocker search [OPTIONS] TERM"))

	outSearchCmd, _ := dockerCmd(c, "search", "busybox")
	outSearchCmdNotrunc, _ := dockerCmd(c, "search", "--no-trunc=true", "busybox")

	assert.Assert(c, len(outSearchCmd) <= len(outSearchCmdNotrunc), "The no-trunc option can't take effect.")

	outSearchCmdautomated, _ := dockerCmd(c, "search", "--filter", "is-automated=true", "busybox") //The busybox is a busybox base image, not an AUTOMATED image.
	outSearchCmdautomatedSlice := strings.Split(outSearchCmdautomated, "\n")
	for i := range outSearchCmdautomatedSlice {
		assert.Assert(c, !strings.HasPrefix(outSearchCmdautomatedSlice[i], "busybox "), "The busybox is not an AUTOMATED image: %s", outSearchCmdautomated)
	}

	outSearchCmdNotOfficial, _ := dockerCmd(c, "search", "--filter", "is-official=false", "busybox") //The busybox is a busybox base image, official image.
	outSearchCmdNotOfficialSlice := strings.Split(outSearchCmdNotOfficial, "\n")
	for i := range outSearchCmdNotOfficialSlice {
		assert.Assert(c, !strings.HasPrefix(outSearchCmdNotOfficialSlice[i], "busybox "), "The busybox is not an OFFICIAL image: %s", outSearchCmdNotOfficial)
	}

	outSearchCmdOfficial, _ := dockerCmd(c, "search", "--filter", "is-official=true", "busybox") //The busybox is a busybox base image, official image.
	outSearchCmdOfficialSlice := strings.Split(outSearchCmdOfficial, "\n")
	assert.Equal(c, len(outSearchCmdOfficialSlice), 3) // 1 header, 1 line, 1 carriage return
	assert.Assert(c, strings.HasPrefix(outSearchCmdOfficialSlice[1], "busybox "), "The busybox is an OFFICIAL image: %s", outSearchCmdOfficial)

	outSearchCmdStars, _ := dockerCmd(c, "search", "--filter", "stars=2", "busybox")
	assert.Assert(c, strings.Count(outSearchCmdStars, "[OK]") <= strings.Count(outSearchCmd, "[OK]"), "The quantity of images with stars should be less than that of all images: %s", outSearchCmdStars)

	dockerCmd(c, "search", "--filter", "is-automated=true", "--filter", "stars=2", "--no-trunc=true", "busybox")

	// --automated deprecated since Docker 1.13
	outSearchCmdautomated1, _ := dockerCmd(c, "search", "--automated=true", "busybox") //The busybox is a busybox base image, not an AUTOMATED image.
	outSearchCmdautomatedSlice1 := strings.Split(outSearchCmdautomated1, "\n")
	for i := range outSearchCmdautomatedSlice1 {
		assert.Assert(c, !strings.HasPrefix(outSearchCmdautomatedSlice1[i], "busybox "), "The busybox is not an AUTOMATED image: %s", outSearchCmdautomated)
	}

	// -s --stars deprecated since Docker 1.13
	outSearchCmdStars1, _ := dockerCmd(c, "search", "--stars=2", "busybox")
	assert.Assert(c, strings.Count(outSearchCmdStars1, "[OK]") <= strings.Count(outSearchCmd, "[OK]"), "The quantity of images with stars should be less than that of all images: %s", outSearchCmdStars1)

	// -s --stars deprecated since Docker 1.13
	dockerCmd(c, "search", "--stars=2", "--automated=true", "--no-trunc=true", "busybox")
}

// search for repos which start with "ubuntu-" on the central registry
func (s *DockerSuite) TestSearchOnCentralRegistryWithDash(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	dockerCmd(c, "search", "ubuntu-")
}

// test case for #23055
func (s *DockerSuite) TestSearchWithLimit(c *check.C) {
	testRequires(c, Network, DaemonIsLinux)

	limit := 10
	out, _, err := dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
	assert.NilError(c, err)
	outSlice := strings.Split(out, "\n")
	assert.Equal(c, len(outSlice), limit+2) // 1 header, 1 carriage return

	limit = 50
	out, _, err = dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
	assert.NilError(c, err)
	outSlice = strings.Split(out, "\n")
	assert.Equal(c, len(outSlice), limit+2) // 1 header, 1 carriage return

	limit = 100
	out, _, err = dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
	assert.NilError(c, err)
	outSlice = strings.Split(out, "\n")
	assert.Equal(c, len(outSlice), limit+2) // 1 header, 1 carriage return

	limit = 0
	_, _, err = dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
	assert.ErrorContains(c, err, "")

	limit = 200
	_, _, err = dockerCmdWithError("search", fmt.Sprintf("--limit=%d", limit), "docker")
	assert.ErrorContains(c, err, "")
}
