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
	c.Assert(regexp.MustCompile(`Pulling.*from.*library/hello-world\b`).MatchString(out), checker.Equals, true, check.Commentf("expected the 'library/' prefix to be automatically assumed in: %s", out))
	c.Assert(out, checker.Contains, "Downloaded newer image for docker.io/hello-world:latest")

	matches := regexp.MustCompile(`Digest: (.+)\n`).FindAllStringSubmatch(out, -1)
	c.Assert(len(matches), checker.Equals, 1, check.Commentf("expected exactly one image digest in the output: %s", out))
	c.Assert(len(matches[0]), checker.Equals, 2, check.Commentf("unexpected number of submatches for the digest"))
	_, err := digest.ParseDigest(matches[0][1])
	c.Check(err, checker.IsNil, check.Commentf("invalid digest %q in output", matches[0][1]))

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	if splitImg := strings.Split(img, "\n"); len(splitImg) != 2 {
		c.Fatalf("expected only two lines in the output of `docker images`, got %d", len(splitImg))
	} else if re := regexp.MustCompile(`^docker\.io/hello-world\s+latest`); !re.Match([]byte(splitImg[1])) {
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
		c.Assert(out, checker.Contains, "Image is up to date for docker.io/hello-world:latest")
	}

	// We should have a single entry in images.
	img := strings.TrimSpace(s.Cmd(c, "images"))
	if splitImg := strings.Split(img, "\n"); len(splitImg) != 2 {
		c.Fatalf("expected only two lines in the output of `docker images`, got %d", len(splitImg))
	} else if re := regexp.MustCompile(`^docker\.io/hello-world\s+latest`); !re.Match([]byte(splitImg[1])) {
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
		if strings.HasPrefix(line, "docker.io/busybox") && strings.Contains(line, "latest") {
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

func (s *DockerRegistrySuite) TestPullFromAdditionalRegistry(c *check.C) {
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

// Test pulls from blocked public registry and from private registry. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func (s *DockerRegistrySuite) doTestPullFromBlockedPublicRegistry(c *check.C, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	busyboxID := s.d.getAndTestImageEntry(c, 1, "busybox", "").id

	// try to pull from docker.io
	if out, err := s.d.Cmd("pull", "library/hello-world"); err == nil {
		c.Fatalf("pull from blocked public registry should have failed, output: %s", out)
	}

	// tag busybox as library/hello-world and push it to some private registry
	if out, err := s.d.Cmd("tag", "busybox", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg.url+"/library/hello-world"); !allBlocked && err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg.url+"/library/hello-world", err, out)
	} else if allBlocked && err == nil {
		c.Fatalf("push to private registry should have failed, output: %q", out)
	}

	// remove library/hello-world image
	if out, err := s.d.Cmd("rmi", s.reg.url+"/library/hello-world"); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", s.reg.url+"/library/hello-world", err, out)
	}
	s.d.getAndTestImageEntry(c, 1, "busybox", busyboxID)

	// try to pull from private registry
	if out, err := s.d.Cmd("pull", s.reg.url+"/library/hello-world"); !allBlocked && err != nil {
		c.Fatalf("we should have been able to pull %s/library/hello-world: %v", s.reg.url, err)
	} else if allBlocked && err == nil {
		c.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		s.d.getAndTestImageEntry(c, 2, s.reg.url+"/library/hello-world", busyboxID)
	}
}

func (s *DockerRegistrySuite) TestPullFromBlockedPublicRegistry(c *check.C) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		s.doTestPullFromBlockedPublicRegistry(c, []string{"--block-registry=" + blockedRegistry})
		s.d.Stop()
		s.d = NewDaemon(c)
	}
}

func (s *DockerRegistrySuite) TestPullWithAllRegistriesBlocked(c *check.C) {
	s.doTestPullFromBlockedPublicRegistry(c, []string{"--block-registry=all"})
}

// Test pulls from additional registry with public registry blocked. This
// shall be called with various daemonArgs containing at least one
// `--block-registry` flag.
func (s *DockerRegistriesSuite) doTestPullFromPrivateRegistriesWithPublicBlocked(c *check.C, daemonArgs []string) {
	allBlocked := false
	for _, arg := range daemonArgs {
		if arg == "--block-registry=all" {
			allBlocked = true
		}
	}
	daemonArgs = append(daemonArgs, "--add-registry="+s.reg1.url)
	if err := s.d.StartWithBusybox(daemonArgs...); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing { %s } flags: %v", strings.Join(daemonArgs, ", "), err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// try to pull from blocked public registry
	if out, err := s.d.Cmd("pull", "library/hello-world"); err == nil {
		c.Fatalf("pulling from blocked public registry should have failed, output: %s", out)
	}

	// push busybox to
	//  additional registry as "misc/busybox"
	//  private registry as "library/busybox"
	// and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", s.reg1.url+"/misc/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg2.url+"/library/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg1.url+"/misc/busybox"); err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg1.url+"/misc/busybox", err, out)
	}
	if out, err := s.d.Cmd("push", s.reg2.url+"/library/busybox"); !allBlocked && err != nil {
		c.Fatalf("failed to push image %s: error %v, output %q", s.reg2.url+"/library/busybox", err, out)
	} else if allBlocked && err == nil {
		c.Fatalf("push to private registry should have failed, output: %q", out)
	}
	toRemove := []string{"busybox", "misc/busybox", s.reg2.url + "/library/busybox"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// try to pull "library/busybox" from additional registry
	if out, err := s.d.Cmd("pull", "library/busybox"); err == nil {
		c.Fatalf("pull of library/busybox from additional registry should have failed, output: %q", out)
	}

	// now pull the "misc/busybox" from additional registry
	if _, err := s.d.Cmd("pull", "misc/busybox"); err != nil {
		c.Fatalf("we should have been able to pull misc/hello-world from %q: %v", s.reg1.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 1, s.reg1.url+"/misc/busybox", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}

	// try to pull "library/busybox" from private registry
	if out, err := s.d.Cmd("pull", s.reg2.url+"/library/busybox"); !allBlocked && err != nil {
		c.Fatalf("we should have been able to pull %s/library/busybox: %v", s.reg2.url, err)
	} else if allBlocked && err == nil {
		c.Fatalf("pull from private registry should have failed, output: %q", out)
	} else if !allBlocked {
		bb3Img := s.d.getAndTestImageEntry(c, 2, s.reg2.url+"/library/busybox", "")
		if bb3Img.size != bbImg.size {
			c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb3Img.name, bbImg.name, bb3Img.size, bbImg.size)
		}
	}
}

func (s *DockerRegistriesSuite) TestPullFromPrivateRegistriesWithPublicBlocked(c *check.C) {
	for _, blockedRegistry := range []string{"public", "docker.io"} {
		s.doTestPullFromPrivateRegistriesWithPublicBlocked(c, []string{"--block-registry=" + blockedRegistry})
		if s.d != nil {
			s.d.Stop()
		}
		s.d = NewDaemon(c)
	}
}

func (s *DockerRegistriesSuite) TestPullFromAdditionalRegistryWithAllBlocked(c *check.C) {
	s.doTestPullFromPrivateRegistriesWithPublicBlocked(c, []string{"--block-registry=all"})
}

func (s *DockerRegistriesSuite) TestPullFromBlockedRegistry(c *check.C) {
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
