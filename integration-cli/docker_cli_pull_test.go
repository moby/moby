package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
		Image string
		Alias string
	}{
		{"library/asdfasdf:foobar", "asdfasdf:foobar"},
		{"library/asdfasdf:foobar", "library/asdfasdf:foobar"},
		{"library/asdfasdf:latest", "asdfasdf"},
		{"library/asdfasdf:latest", "asdfasdf:latest"},
		{"library/asdfasdf:latest", "library/asdfasdf"},
		{"library/asdfasdf:latest", "library/asdfasdf:latest"},
	} {
		out, err := s.CmdWithError("pull", e.Alias)
		c.Assert(err, checker.NotNil, check.Commentf("expected non-zero exit status when pulling non-existing image: %s", out))
		c.Assert(out, checker.Contains, fmt.Sprintf("Error: image %s not found", e.Image), check.Commentf("expected image not found error messages"))
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

type idAndParent struct {
	ID     string
	Parent string
}

func inspectImage(c *check.C, imageRef string) idAndParent {
	out, _ := dockerCmd(c, "inspect", imageRef)
	var inspectOutput []idAndParent
	err := json.Unmarshal([]byte(out), &inspectOutput)
	if err != nil {
		c.Fatal(err)
	}

	return inspectOutput[0]
}

func imageID(c *check.C, imageRef string) string {
	return inspectImage(c, imageRef).ID
}

func imageParent(c *check.C, imageRef string) string {
	return inspectImage(c, imageRef).Parent
}

// TestPullMigration verifies that pulling an image based on layers
// that already exists locally will reuse those existing layers.
func (s *DockerRegistrySuite) TestPullMigration(c *check.C) {
	repoName := privateRegistryURL + "/dockercli/migration"

	baseImage := repoName + ":base"
	_, err := buildImage(baseImage, fmt.Sprintf(`
	    FROM scratch
	    ENV IMAGE base
	    CMD echo %s
	`, baseImage), true)
	if err != nil {
		c.Fatal(err)
	}

	baseIDBeforePush := imageID(c, baseImage)
	baseParentBeforePush := imageParent(c, baseImage)

	derivedImage := repoName + ":derived"
	_, err = buildImage(derivedImage, fmt.Sprintf(`
	    FROM %s
	    CMD echo %s
	`, baseImage, derivedImage), true)
	if err != nil {
		c.Fatal(err)
	}

	derivedIDBeforePush := imageID(c, derivedImage)

	dockerCmd(c, "push", derivedImage)

	// Remove derived image from the local store
	dockerCmd(c, "rmi", derivedImage)

	// Repull
	dockerCmd(c, "pull", derivedImage)

	// Check that the parent of this pulled image is the original base
	// image
	derivedIDAfterPull1 := imageID(c, derivedImage)
	derivedParentAfterPull1 := imageParent(c, derivedImage)

	if derivedIDAfterPull1 == derivedIDBeforePush {
		c.Fatal("image's ID should have changed on after deleting and pulling")
	}

	if derivedParentAfterPull1 != baseIDBeforePush {
		c.Fatalf("pulled image's parent ID (%s) does not match base image's ID (%s)", derivedParentAfterPull1, baseIDBeforePush)
	}

	// Confirm that repushing and repulling does not change the computed ID
	dockerCmd(c, "push", derivedImage)
	dockerCmd(c, "rmi", derivedImage)
	dockerCmd(c, "pull", derivedImage)

	derivedIDAfterPull2 := imageID(c, derivedImage)
	derivedParentAfterPull2 := imageParent(c, derivedImage)

	if derivedIDAfterPull2 != derivedIDAfterPull1 {
		c.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	if derivedParentAfterPull2 != baseIDBeforePush {
		c.Fatalf("pulled image's parent ID (%s) does not match base image's ID (%s)", derivedParentAfterPull2, baseIDBeforePush)
	}

	// Remove everything, repull, and make sure everything uses computed IDs
	dockerCmd(c, "rmi", baseImage, derivedImage)
	dockerCmd(c, "pull", derivedImage)

	derivedIDAfterPull3 := imageID(c, derivedImage)
	derivedParentAfterPull3 := imageParent(c, derivedImage)
	derivedGrandparentAfterPull3 := imageParent(c, derivedParentAfterPull3)

	if derivedIDAfterPull3 != derivedIDAfterPull1 {
		c.Fatal("image's ID unexpectedly changed after a second repull")
	}

	if derivedParentAfterPull3 == baseIDBeforePush {
		c.Fatalf("pulled image's parent ID (%s) should not match base image's original ID (%s)", derivedParentAfterPull3, derivedIDBeforePush)
	}

	if derivedGrandparentAfterPull3 == baseParentBeforePush {
		c.Fatal("base image's parent ID should have been rewritten on pull")
	}
}

// TestPullMigrationRun verifies that pulling an image based on layers
// that already exists locally will result in an image that runs properly.
func (s *DockerRegistrySuite) TestPullMigrationRun(c *check.C) {
	type idAndParent struct {
		ID     string
		Parent string
	}

	derivedImage := privateRegistryURL + "/dockercli/migration-run"
	baseImage := "busybox"

	_, err := buildImage(derivedImage, fmt.Sprintf(`
	    FROM %s
	    RUN dd if=/dev/zero of=/file bs=1024 count=1024
	    CMD echo %s
	`, baseImage, derivedImage), true)
	if err != nil {
		c.Fatal(err)
	}

	baseIDBeforePush := imageID(c, baseImage)
	derivedIDBeforePush := imageID(c, derivedImage)

	dockerCmd(c, "push", derivedImage)

	// Remove derived image from the local store
	dockerCmd(c, "rmi", derivedImage)

	// Repull
	dockerCmd(c, "pull", derivedImage)

	// Check that this pulled image is based on the original base image
	derivedIDAfterPull1 := imageID(c, derivedImage)
	derivedParentAfterPull1 := imageParent(c, imageParent(c, derivedImage))

	if derivedIDAfterPull1 == derivedIDBeforePush {
		c.Fatal("image's ID should have changed on after deleting and pulling")
	}

	if derivedParentAfterPull1 != baseIDBeforePush {
		c.Fatalf("pulled image's parent ID (%s) does not match base image's ID (%s)", derivedParentAfterPull1, baseIDBeforePush)
	}

	// Make sure the image runs correctly
	out, _ := dockerCmd(c, "run", "--rm", derivedImage)
	if strings.TrimSpace(out) != derivedImage {
		c.Fatalf("expected %s; got %s", derivedImage, out)
	}

	// Confirm that repushing and repulling does not change the computed ID
	dockerCmd(c, "push", derivedImage)
	dockerCmd(c, "rmi", derivedImage)
	dockerCmd(c, "pull", derivedImage)

	derivedIDAfterPull2 := imageID(c, derivedImage)
	derivedParentAfterPull2 := imageParent(c, imageParent(c, derivedImage))

	if derivedIDAfterPull2 != derivedIDAfterPull1 {
		c.Fatal("image's ID unexpectedly changed after a repush/repull")
	}

	if derivedParentAfterPull2 != baseIDBeforePush {
		c.Fatalf("pulled image's parent ID (%s) does not match base image's ID (%s)", derivedParentAfterPull2, baseIDBeforePush)
	}

	// Make sure the image still runs
	out, _ = dockerCmd(c, "run", "--rm", derivedImage)
	if strings.TrimSpace(out) != derivedImage {
		c.Fatalf("expected %s; got %s", derivedImage, out)
	}
}

// TestPullConflict provides coverage of the situation where a computed
// strongID conflicts with some unverifiable data in the graph.
func (s *DockerRegistrySuite) TestPullConflict(c *check.C) {
	repoName := privateRegistryURL + "/dockercli/conflict"

	_, err := buildImage(repoName, `
	    FROM scratch
	    ENV IMAGE conflict
	    CMD echo conflict
	`, true)
	if err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "push", repoName)

	// Pull to make it content-addressable
	dockerCmd(c, "rmi", repoName)
	dockerCmd(c, "pull", repoName)

	IDBeforeLoad := imageID(c, repoName)

	// Load/save to turn this into an unverified image with the same ID
	tmpDir, err := ioutil.TempDir("", "conflict-save-output")
	if err != nil {
		c.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	tarFile := filepath.Join(tmpDir, "repo.tar")

	dockerCmd(c, "save", "-o", tarFile, repoName)
	dockerCmd(c, "rmi", repoName)
	dockerCmd(c, "load", "-i", tarFile)

	// Check that the the ID is the same after save/load.
	IDAfterLoad := imageID(c, repoName)

	if IDAfterLoad != IDBeforeLoad {
		c.Fatal("image's ID should be the same after save/load")
	}

	// Repull
	dockerCmd(c, "pull", repoName)

	// Check that the ID is now different because of the conflict.
	IDAfterPull1 := imageID(c, repoName)

	// Expect the new ID to be SHA256(oldID)
	expectedIDDigest, err := digest.FromBytes([]byte(IDBeforeLoad))
	if err != nil {
		c.Fatalf("digest error: %v", err)
	}
	expectedID := expectedIDDigest.Hex()
	if IDAfterPull1 != expectedID {
		c.Fatalf("image's ID should have changed on pull to %s (got %s)", expectedID, IDAfterPull1)
	}

	// A second pull should use the new ID again.
	dockerCmd(c, "pull", repoName)

	IDAfterPull2 := imageID(c, repoName)

	if IDAfterPull2 != IDAfterPull1 {
		c.Fatal("image's ID unexpectedly changed after a repull")
	}
}
