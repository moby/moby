package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type DockerCLIHistorySuite struct {
	ds *DockerSuite
}

func (s *DockerCLIHistorySuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIHistorySuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// This is a heisen-test.  Because the created timestamp of images and the behavior of
// sort is not predictable it doesn't always fail.
func (s *DockerCLIHistorySuite) TestBuildHistory(c *testing.T) {
	const name = "testbuildhistory"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM `+minimalBaseImage()+`
LABEL label.A="A"
LABEL label.B="B"
LABEL label.C="C"
LABEL label.D="D"
LABEL label.E="E"
LABEL label.F="F"
LABEL label.G="G"
LABEL label.H="H"
LABEL label.I="I"
LABEL label.J="J"
LABEL label.K="K"
LABEL label.L="L"
LABEL label.M="M"
LABEL label.N="N"
LABEL label.O="O"
LABEL label.P="P"
LABEL label.Q="Q"
LABEL label.R="R"
LABEL label.S="S"
LABEL label.T="T"
LABEL label.U="U"
LABEL label.V="V"
LABEL label.W="W"
LABEL label.X="X"
LABEL label.Y="Y"
LABEL label.Z="Z"`))

	out := cli.DockerCmd(c, "history", name).Combined()
	actualValues := strings.Split(out, "\n")[1:27]
	expectedValues := [26]string{"Z", "Y", "X", "W", "V", "U", "T", "S", "R", "Q", "P", "O", "N", "M", "L", "K", "J", "I", "H", "G", "F", "E", "D", "C", "B", "A"}

	for i := 0; i < 26; i++ {
		echoValue := fmt.Sprintf("LABEL label.%s=%s", expectedValues[i], expectedValues[i])
		actualValue := actualValues[i]
		assert.Assert(c, strings.Contains(actualValue, echoValue))
	}
}

func (s *DockerCLIHistorySuite) TestHistoryExistentImage(c *testing.T) {
	cli.DockerCmd(c, "history", "busybox")
}

func (s *DockerCLIHistorySuite) TestHistoryNonExistentImage(c *testing.T) {
	_, _, err := dockerCmdWithError("history", "testHistoryNonExistentImage")
	assert.Assert(c, err != nil, "history on a non-existent image should fail.")
}

func (s *DockerCLIHistorySuite) TestHistoryImageWithComment(c *testing.T) {
	const name = "testhistoryimagewithcomment"

	// make an image through docker commit <container id> [ -m messages ]
	cli.DockerCmd(c, "run", "--name", name, "busybox", "true")
	cli.DockerCmd(c, "wait", name)

	const comment = "This_is_a_comment"
	cli.DockerCmd(c, "commit", "-m="+comment, name, name)

	// test docker history <image id> to check comment messages
	out := cli.DockerCmd(c, "history", name).Combined()
	outputTabs := strings.Fields(strings.Split(out, "\n")[1])
	actualValue := outputTabs[len(outputTabs)-1]
	assert.Assert(c, strings.Contains(actualValue, comment))
}

func (s *DockerCLIHistorySuite) TestHistoryHumanOptionFalse(c *testing.T) {
	out := cli.DockerCmd(c, "history", "--human=false", "busybox").Combined()
	lines := strings.Split(out, "\n")
	sizeColumnRegex, _ := regexp.Compile("SIZE +")
	indices := sizeColumnRegex.FindStringIndex(lines[0])
	startIndex := indices[0]
	endIndex := indices[1]
	for i := 1; i < len(lines)-1; i++ {
		if endIndex > len(lines[i]) {
			endIndex = len(lines[i])
		}
		sizeString := lines[i][startIndex:endIndex]

		_, err := strconv.Atoi(strings.TrimSpace(sizeString))
		assert.Assert(c, err == nil, "The size '%s' was not an Integer", sizeString)
	}
}

func (s *DockerCLIHistorySuite) TestHistoryHumanOptionTrue(c *testing.T) {
	out := cli.DockerCmd(c, "history", "--human=true", "busybox").Combined()
	lines := strings.Split(out, "\n")
	sizeColumnRegex, _ := regexp.Compile("SIZE +")
	humanSizeRegexRaw := "\\d+.*B" // Matches human sizes like 10 MB, 3.2 KB, etc
	indices := sizeColumnRegex.FindStringIndex(lines[0])
	startIndex := indices[0]
	endIndex := indices[1]
	for i := 1; i < len(lines)-1; i++ {
		if endIndex > len(lines[i]) {
			endIndex = len(lines[i])
		}
		sizeString := lines[i][startIndex:endIndex]
		assert.Assert(c, cmp.Regexp("^"+humanSizeRegexRaw+"$",
			strings.TrimSpace(sizeString)), fmt.Sprintf("The size '%s' was not in human format", sizeString))
	}
}
