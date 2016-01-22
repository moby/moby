package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/registry"
	"github.com/go-check/check"
)

// TestPullFromCentralRegistry pulls an image from the central registry and verifies that the client
// prints all expected output.
func (s *DockerHubPullSuite) TestPullFromCentralRegistry(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out := s.Cmd(c, "pull", "hello-world")
	defer deleteImages("hello-world")

	c.Assert(out, checker.Contains, "Using default tag: latest", check.Commentf("expected the 'latest' tag to be automatically assumed"))
	c.Assert(regexp.MustCompile(`Pulling.*from.*library/hello-world\b`).MatchString(out), checker.Equals, true, check.Commentf("expected the 'library/' prefix to be automatically assumed in: %s", out))
	c.Assert(out, checker.Contains, "Downloaded newer image for docker.io/hello-world:latest")

	matches := regexp.MustCompile(`Digest: (.+)\n`).FindAllStringSubmatch(out, -1)
	c.Assert(len(matches), checker.Equals, 1, check.Commentf("expected exactly one image digest in the output: %s", out))
	c.Assert(len(matches[0]), checker.Equals, 2, check.Commentf("unexpected number of submatches for the digest"))
	_, err := digest.ParseDigest(matches[0][1])
	c.Check(err, checker.IsNil, check.Commentf("invalid digest %q in output", matches[0][1]))

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	splitImg := strings.Split(img, "\n")
	c.Assert(splitImg, checker.HasLen, 2)
	c.Assert(splitImg[1], checker.Matches, `docker\.io/hello-world\s+latest.*?`, check.Commentf("invalid output for `docker images` (expected image and tag name"))
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
		// Hub returns 401 rather than 404 for nonexistent repos over
		// the v2 protocol - but we should end up falling back to v1,
		// which does return a 404.
		c.Assert(out, checker.Contains, fmt.Sprintf("Error: image %s not found", e.Repo), check.Commentf("expected image not found error messages"))

		// pull -a on a nonexistent registry should fall back as well
		if !strings.ContainsRune(e.Alias, ':') {
			out, err := s.CmdWithError("pull", "-a", e.Alias)
			c.Assert(err, checker.NotNil, check.Commentf("expected non-zero exit status when pulling non-existing image: %s", out))
			c.Assert(out, checker.Contains, fmt.Sprintf("Error: image %s not found", e.Repo), check.Commentf("expected image not found error messages"))
		}
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
		c.Assert(out, checker.Contains, "Image is up to date for docker.io/hello-world:latest")
	}

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	splitImg := strings.Split(img, "\n")
	c.Assert(splitImg, checker.HasLen, 2)
	c.Assert(splitImg[1], checker.Matches, `docker\.io/hello-world\s+latest.*?`, check.Commentf("invalid output for `docker images` (expected image and tag name"))
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
	c.Assert(splitOutImageCmd, checker.HasLen, 2)

	s.Cmd(c, "pull", "--all-tags=true", "busybox")
	outImageAllTagCmd := s.Cmd(c, "images", "busybox")
	linesCount := strings.Count(outImageAllTagCmd, "\n")
	c.Assert(linesCount, checker.GreaterThan, 2, check.Commentf("pulling all tags should provide more than two images, got %s", outImageAllTagCmd))

	// Verify that the line for 'busybox:latest' is left unchanged.
	var latestLine string
	for _, line := range strings.Split(outImageAllTagCmd, "\n") {
		if strings.HasPrefix(line, "docker.io/busybox") && strings.Contains(line, "latest") {
			latestLine = line
			break
		}
	}
	c.Assert(latestLine, checker.Not(checker.Equals), "", check.Commentf("no entry for busybox:latest found after pulling all tags"))
	splitLatest := strings.Fields(latestLine)
	splitCurrent := strings.Fields(splitOutImageCmd[1])

	// Clear relative creation times, since these can easily change between
	// two invocations of "docker images". Without this, the test can fail
	// like this:
	// ... obtained []string = []string{"busybox", "latest", "d9551b4026f0", "27", "minutes", "ago", "1.113", "MB"}
	// ... expected []string = []string{"busybox", "latest", "d9551b4026f0", "26", "minutes", "ago", "1.113", "MB"}
	splitLatest[3] = ""
	splitLatest[4] = ""
	splitLatest[5] = ""
	splitCurrent[3] = ""
	splitCurrent[4] = ""
	splitCurrent[5] = ""

	c.Assert(splitLatest, checker.DeepEquals, splitCurrent, check.Commentf("busybox:latest was changed after pulling all tags"))
}

// TestPullClientDisconnect kills the client during a pull operation and verifies that the operation
// gets cancelled.
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

	time.Sleep(2 * time.Second)
	_, err = s.CmdWithError("inspect", repoName)
	c.Assert(err, checker.NotNil, check.Commentf("image was pulled after client disconnected"))
}

func (s *DockerRegistrySuite) TestPullFromAdditionalRegistry(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, Network)

	if err := s.d.StartWithBusybox("--add-registry=" + s.reg.url); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", s.reg.url, err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// this will pull from docker.io
	if _, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from %q: %v", s.reg.url, err)
	}

	hwImg := s.d.getAndTestImageEntry(c, 2, "docker.io/hello-world", "")
	if hwImg.id == bbImg.id || hwImg.size == bbImg.size {
		c.Fatalf("docker.io/hello-world must have different ID and size than busybox image")
	}

	// push busybox to additional registry as "library/hello-world" and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg.url+"/library/hello-world", err, out)
	}
	toRemove := []string{"library/hello-world", "busybox", "docker.io/hello-world"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// pull the same name again - now the image should be pulled from additional registry
	if _, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from %q: %v", s.reg.url, err)
	}
	hw2Img := s.d.getAndTestImageEntry(c, 1, s.reg.url+"/library/hello-world", "")
	if hw2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", hw2Img.name, bbImg.name, hw2Img.size, hwImg.size)
	}

	// empty images once more
	if out, err := s.d.Cmd("rmi", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to remove image %s: %v, output: %s", s.reg.url+"library/hello-world", err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// now pull with fully qualified name
	if _, err := s.d.Cmd("pull", "docker.io/library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull docker.io/library/hello-world from %q: %v", s.reg.url, err)
	}
	hw3Img := s.d.getAndTestImageEntry(c, 1, "docker.io/hello-world", "")
	if hw3Img.size != hwImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", hw3Img.name, hwImg.name, hw3Img.size, hwImg.size)
	}
}

func (s *DockerRegistriesSuite) TestPullFromAdditionalRegistries(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, Network)

	daemonArgs := []string{"--add-registry=" + s.reg1.url, "--add-registry=" + s.reg2.url}
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// this will pull from docker.io
	if out, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from \"docker.io\": %s\n\nError:%v", out, err)
	}
	hwImg := s.d.getAndTestImageEntry(c, 2, "docker.io/hello-world", "")
	if hwImg.id == bbImg.id {
		c.Fatalf("docker.io/hello-world must have different ID than busybox image")
	}

	// push:
	//  hello-world to 1st additional registry as "misc/hello-world"
	//  busybox to 2nd additional registry as "library/hello-world"
	if out, err := s.d.Cmd("tag", "docker.io/hello-world", s.reg1.url+"/misc/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "docker.io/hello-world", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "/busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg1.url+"/misc/hello-world"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg1.url+"/misc/hello-world", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg2.url+"/library/busybox", err, out)
	}
	// and remove all local images
	toRemove := []string{"misc/hello-world", s.reg2.url + "/library/hello-world", "busybox", "docker.io/hello-world"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// now pull the "library/hello-world" from 2nd additional registry
	if _, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from %q: %v", s.reg2.url, err)
	}
	hw2Img := s.d.getAndTestImageEntry(c, 1, s.reg2.url+"/library/hello-world", "")
	if hw2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", hw2Img.name, bbImg.name, hw2Img.size, bbImg.size)
	}

	// now pull the "misc/hello-world" from 1st additional registry
	if _, err := s.d.Cmd("pull", "misc/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull misc/hello-world from %q: %v", s.reg2.url, err)
	}
	hw3Img := s.d.getAndTestImageEntry(c, 2, s.reg1.url+"/misc/hello-world", "")
	if hw3Img.size != hwImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", hw3Img.name, hwImg.name, hw3Img.size, hwImg.size)
	}

	// tag it as library/hello-world and push it to 1st registry
	if out, err := s.d.Cmd("tag", s.reg1.url+"/misc/hello-world", s.reg1.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", s.reg1.url+"/misc/hello-world", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg1.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg1.url+"/library/hello-world", err, out)
	}

	// remove all images
	toRemove = []string{s.reg1.url + "/misc/hello-world", s.reg1.url + "/library/hello-world", s.reg2.url + "/library/hello-world"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// now pull "library/hello-world" from 1st additional registry
	if _, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from %q: %v", s.reg1.url, err)
	}
	hw4Img := s.d.getAndTestImageEntry(c, 1, s.reg1.url+"/library/hello-world", "")
	if hw4Img.size != hwImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", hw4Img.name, hwImg.name, hw4Img.size, hwImg.size)
	}

	// now pull fully qualified image from 2nd registry
	if _, err := s.d.Cmd("pull", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull %s/library/hello-world: %v", s.reg2.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 2, s.reg2.url+"/library/hello-world", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}
}

func (s *DockerRegistriesSuite) TestPullFromBlockedRegistry(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, Network)

	daemonArgs := []string{"--block-registry=" + s.reg1.url, "--add-registry=" + s.reg2.url}
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// pull image from docker.io
	if _, err := s.d.Cmd("pull", "library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from \"docker.io\": %v", err)
	}
	hwImg := s.d.getAndTestImageEntry(c, 2, "docker.io/hello-world", "")
	if hwImg.size == bbImg.size {
		c.Fatalf("docker.io/hello-world must have different size than busybox image")
	}

	// push "hello-world" to blocked and additional registry and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", s.reg1.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg1.url+"/library/hello-world"); err == nil {
		c.Fatalf("push to blocked registry should have failed, output: %q", out)
	}
	if out, err := s.d.Cmd("push", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg2.url+"/library/hello-world", err, out)
	}
	toRemove := []string{"library/hello-world", s.reg1.url + "/library/hello-world", "docker.io/hello-world", "busybox"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// try to pull "library/hello-world" from blocked registry
	if out, err := s.d.Cmd("pull", s.reg1.url+"/library/hello-world"); err == nil {
		c.Fatalf("pull of library/hello-world from additional registry should have failed, output: %q", out)
	}

	// now pull the "library/hello-world" from additional registry
	if _, err := s.d.Cmd("pull", s.reg2.url+"/library/hello-world"); err != nil {
		c.Fatalf("we should have been able to pull library/hello-world from %q: %v", s.reg2.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 1, s.reg2.url+"/library/hello-world", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}
}

func (s *DockerRegistriesSuite) TestPullNeedsAuth(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--add-registry="+s.regWithAuth.url), check.IsNil)

	repo := fmt.Sprintf("%s/runcom/busybox", s.regWithAuth.url)
	repoUnqualified := "runcom/busybox"

	out, err := s.d.Cmd("tag", "busybox", repo)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// this means it needs auth...
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", s.regWithAuth.url))
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusUnauthorized)

	// login with the registry...
	out, err = s.d.Cmd("login", "-u", s.regWithAuth.username, "-p", s.regWithAuth.password, "-e", s.regWithAuth.email, s.regWithAuth.url)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// push the image so we can pull from unqualified "runcom/busybox"
	out, err = s.d.Cmd("push", repo)
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("rmi", "-f", repoUnqualified)
	c.Assert(err, check.IsNil, check.Commentf(out))

	out, err = s.d.Cmd("pull", repoUnqualified)
	c.Assert(err, check.IsNil, check.Commentf(out))
	// check it's pulling from private reg with auth with unqualified images and not docker.io
	if strings.Contains(out, registry.DefaultNamespace) {
		c.Fatalf("Expected not to contact docker.io, got %s", out)
	}
	expected := fmt.Sprintf("Pulling from %s", repo)
	if !strings.Contains(out, expected) {
		c.Fatalf("Wanted %s, got %s", expected, out)
	}

	out, err = s.d.Cmd("images")
	c.Assert(err, check.IsNil, check.Commentf(out))
	// docker images shows correct registry prefixed image
	if !strings.Contains(out, repo) {
		c.Fatalf("Wanted %s in output, got %s", repo, out)
	}
}
