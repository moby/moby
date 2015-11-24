package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// TestPullFromCentralRegistry pulls an image from the central registry and verifies that the client
// prints all expected output.
func (s *DockerHubPullSuite) TestPullFromCentralRegistry(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out := s.Cmd(c, "pull", "hello-world")
	defer deleteImages("hello-world")

	c.Assert(out, checker.Contains, "Using default tag: latest", check.Commentf("expected the 'latest' tag to be automatically assumed"))
	c.Assert(out, checker.Contains, "Pulling from library/hello-world", check.Commentf("expected the 'library/' prefix to be automatically assumed"))
	c.Assert(out, checker.Contains, "Downloaded newer image for hello-world:latest")

	matches := regexp.MustCompile(`Digest: (.+)\n`).FindAllStringSubmatch(out, -1)
	c.Assert(len(matches), checker.Equals, 1, check.Commentf("expected exactly one image digest in the output"))
	c.Assert(len(matches[0]), checker.Equals, 2, check.Commentf("unexpected number of submatches for the digest"))
	_, err := digest.ParseDigest(matches[0][1])
	c.Check(err, checker.IsNil, check.Commentf("invalid digest %q in output", matches[0][1]))

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	if splitImg := strings.Split(img, "\n"); len(splitImg) != 2 {
		c.Fatalf("expected only two lines in the output of `docker images`, got %d", len(splitImg))
	} else if re := regexp.MustCompile(`^hello-world\s+latest`); !re.Match([]byte(splitImg[1])) {
		c.Fatal("invalid output for `docker images` (expected image and tag name")
	}
}

// TestPullNonExistingImage pulls non-existing images from the central registry, with different
// combinations of implicit tag and library prefix.
func (s *DockerHubPullSuite) TestPullNonExistingImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	for _, e := range []struct {
		Repo  string
		Alias string
	}{
		{"library/asdfasdf", "asdfasdf:foobar"},
		{"library/asdfasdf", "library/asdfasdf:foobar"},
		{"library/asdfasdf", "asdfasdf"},
		{"library/asdfasdf", "asdfasdf:latest"},
		{"library/asdfasdf", "library/asdfasdf"},
		{"library/asdfasdf", "library/asdfasdf:latest"},
	} {
		out, err := s.CmdWithError("pull", e.Alias)
		c.Assert(err, checker.NotNil, check.Commentf("expected non-zero exit status when pulling non-existing image: %s", out))
		c.Assert(out, checker.Contains, fmt.Sprintf("Error: image %s not found", e.Repo), check.Commentf("expected image not found error messages"))
	}
}

// TestPullFromCentralRegistryImplicitRefParts pulls an image from the central registry and verifies
// that pulling the same image with different combinations of implicit elements of the the image
// reference (tag, repository, central registry url, ...) doesn't trigger a new pull nor leads to
// multiple images.
func (s *DockerHubPullSuite) TestPullFromCentralRegistryImplicitRefParts(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.Cmd(c, "pull", "hello-world")
	defer deleteImages("hello-world")

	for _, i := range []string{
		"hello-world",
		"hello-world:latest",
		"library/hello-world",
		"library/hello-world:latest",
		"docker.io/library/hello-world",
		"index.docker.io/library/hello-world",
	} {
		out := s.Cmd(c, "pull", i)
		c.Assert(out, checker.Contains, "Image is up to date for hello-world:latest")
	}

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	if splitImg := strings.Split(img, "\n"); len(splitImg) != 2 {
		c.Fatalf("expected only two lines in the output of `docker images`, got %d", len(splitImg))
	} else if re := regexp.MustCompile(`^hello-world\s+latest`); !re.Match([]byte(splitImg[1])) {
		c.Fatal("invalid output for `docker images` (expected image and tag name")
	}
}

// TestPullScratchNotAllowed verifies that pulling 'scratch' is rejected.
func (s *DockerHubPullSuite) TestPullScratchNotAllowed(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, err := s.CmdWithError("pull", "scratch")
	c.Assert(err, checker.NotNil, check.Commentf("expected pull of scratch to fail"))
	c.Assert(out, checker.Contains, "'scratch' is a reserved name")
	c.Assert(out, checker.Not(checker.Contains), "Pulling repository scratch")
}

// TestPullAllTagsFromCentralRegistry pulls using `all-tags` for a given image and verifies that it
// results in more images than a naked pull.
func (s *DockerHubPullSuite) TestPullAllTagsFromCentralRegistry(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.Cmd(c, "pull", "busybox")
	outImageCmd := s.Cmd(c, "images", "busybox")
	splitOutImageCmd := strings.Split(strings.TrimSpace(outImageCmd), "\n")
	c.Assert(splitOutImageCmd, checker.HasLen, 2, check.Commentf("expected a single entry in images\n%v", outImageCmd))

	s.Cmd(c, "pull", "--all-tags=true", "busybox")
	outImageAllTagCmd := s.Cmd(c, "images", "busybox")
	if linesCount := strings.Count(outImageAllTagCmd, "\n"); linesCount <= 2 {
		c.Fatalf("pulling all tags should provide more images, got %d", linesCount-1)
	}

	// Verify that the line for 'busybox:latest' is left unchanged.
	var latestLine string
	for _, line := range strings.Split(outImageAllTagCmd, "\n") {
		if strings.HasPrefix(line, "busybox") && strings.Contains(line, "latest") {
			latestLine = line
			break
		}
	}
	c.Assert(latestLine, checker.Not(checker.Equals), "", check.Commentf("no entry for busybox:latest found after pulling all tags"))
	splitLatest := strings.Fields(latestLine)
	splitCurrent := strings.Fields(splitOutImageCmd[1])
	c.Assert(splitLatest, checker.DeepEquals, splitCurrent, check.Commentf("busybox:latest was changed after pulling all tags"))
}

// TestPullClientDisconnect kills the client during a pull operation and verifies that the operation
// still succesfully completes on the daemon side.
//
// Ref: docker/docker#15589
func (s *DockerHubPullSuite) TestPullClientDisconnect(c *check.C) {
	testRequires(c, DaemonIsLinux)
	repoName := "hello-world:latest"

	pullCmd := s.MakeCmd("pull", repoName)
	stdout, err := pullCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	err = pullCmd.Start()
	c.Assert(err, checker.IsNil)

	// Cancel as soon as we get some output.
	buf := make([]byte, 10)
	_, err = stdout.Read(buf)
	c.Assert(err, checker.IsNil)

	err = pullCmd.Process.Kill()
	c.Assert(err, checker.IsNil)

	maxAttempts := 20
	for i := 0; ; i++ {
		if _, err := s.CmdWithError("inspect", repoName); err == nil {
			break
		}
		if i >= maxAttempts {
			c.Fatal("timeout reached: image was not pulled after client disconnected")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
