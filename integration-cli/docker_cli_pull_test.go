package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/distribution/digest"
	"github.com/go-check/check"
)

// See issue docker/docker#8141
func (s *DockerRegistrySuite) TestPullImageWithAliases(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", repoName, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		dockerCmd(c, "tag", "busybox", repo)
		dockerCmd(c, "push", repo)
	}

	// Clear local images store.
	args := append([]string{"rmi"}, repos...)
	dockerCmd(c, args...)

	// Pull a single tag and verify it doesn't bring down all aliases.
	dockerCmd(c, "pull", repos[0])
	dockerCmd(c, "inspect", repos[0])
	for _, repo := range repos[1:] {
		if _, _, err := dockerCmdWithError(c, "inspect", repo); err == nil {
			c.Fatalf("Image %v shouldn't have been pulled down", repo)
		}
	}
}

// pulling library/hello-world should show verified message
func (s *DockerSuite) TestPullVerified(c *check.C) {
	c.Skip("Skipping hub dependent test")

	// Image must be pulled from central repository to get verified message
	// unless keychain is manually updated to contain the daemon's sign key.

	verifiedName := "hello-world"

	// pull it
	expected := "The image you are pulling has been verified"
	if out, exitCode, err := dockerCmdWithError(c, "pull", verifiedName); err != nil || !strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			c.Skip(fmt.Sprintf("pulling the '%s' image from the registry has failed: %v", verifiedName, err))
		}
		c.Fatalf("pulling a verified image failed. expected: %s\ngot: %s, %v", expected, out, err)
	}

	// pull it again
	if out, exitCode, err := dockerCmdWithError(c, "pull", verifiedName); err != nil || strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			c.Skip(fmt.Sprintf("pulling the '%s' image from the registry has failed: %v", verifiedName, err))
		}
		c.Fatalf("pulling a verified image failed. unexpected verify message\ngot: %s, %v", out, err)
	}

}

// pulling an image from the central registry should work
func (s *DockerSuite) TestPullImageFromCentralRegistry(c *check.C) {
	testRequires(c, Network)

	dockerCmd(c, "pull", "hello-world")
}

// pulling a non-existing image from the central registry should return a non-zero exit code
func (s *DockerSuite) TestPullNonExistingImage(c *check.C) {
	testRequires(c, Network)

	name := "sadfsadfasdf"
	out, _, err := dockerCmdWithError(c, "pull", name)

	if err == nil || !strings.Contains(out, fmt.Sprintf("Error: image library/%s:latest not found", name)) {
		c.Fatalf("expected non-zero exit status when pulling non-existing image: %s", out)
	}
}

// pulling an image from the central registry using official names should work
// ensure all pulls result in the same image
func (s *DockerSuite) TestPullImageOfficialNames(c *check.C) {
	testRequires(c, Network)

	names := []string{
		"library/hello-world",
		"docker.io/library/hello-world",
		"index.docker.io/library/hello-world",
	}
	for _, name := range names {
		out, exitCode, err := dockerCmdWithError(c, "pull", name)
		if err != nil || exitCode != 0 {
			c.Errorf("pulling the '%s' image from the registry has failed: %s", name, err)
			continue
		}

		// ensure we don't have multiple image names.
		out, _ = dockerCmd(c, "images")
		if strings.Contains(out, name) {
			c.Errorf("images should not have listed '%s'", name)
		}
	}
}

func (s *DockerSuite) TestPullScratchNotAllowed(c *check.C) {
	testRequires(c, Network)

	out, exitCode, err := dockerCmdWithError(c, "pull", "scratch")
	if err == nil {
		c.Fatal("expected pull of scratch to fail, but it didn't")
	}
	if exitCode != 1 {
		c.Fatalf("pulling scratch expected exit code 1, got %d", exitCode)
	}
	if strings.Contains(out, "Pulling repository scratch") {
		c.Fatalf("pulling scratch should not have begun: %s", out)
	}
	if !strings.Contains(out, "'scratch' is a reserved name") {
		c.Fatalf("unexpected output pulling scratch: %s", out)
	}
}

// pulling an image with --all-tags=true
func (s *DockerSuite) TestPullImageWithAllTagFromCentralRegistry(c *check.C) {
	testRequires(c, Network)

	dockerCmd(c, "pull", "busybox")

	outImageCmd, _ := dockerCmd(c, "images", "busybox")

	dockerCmd(c, "pull", "--all-tags=true", "busybox")

	outImageAllTagCmd, _ := dockerCmd(c, "images", "busybox")

	if strings.Count(outImageCmd, "busybox") >= strings.Count(outImageAllTagCmd, "busybox") {
		c.Fatalf("Pulling with all tags should get more images")
	}

	// FIXME has probably no effect (tags already pushed)
	dockerCmd(c, "pull", "-a", "busybox")

	outImageAllTagCmd, _ = dockerCmd(c, "images", "busybox")

	if strings.Count(outImageCmd, "busybox") >= strings.Count(outImageAllTagCmd, "busybox") {
		c.Fatalf("Pulling with all tags should get more images")
	}
}

func (s *DockerTrustSuite) TestTrustedPull(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-pull")

	// Try pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try untrusted pull to ensure we pushed the tag to the registry
	pullCmd = exec.Command(dockerBinary, "pull", "--disable-content-trust=true", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Status: Downloaded") {
		c.Fatalf("Missing expected output on trusted pull with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedIsolatedPull(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-isolatd-pull")

	// Try pull (run from isolated directory without trust information)
	pullCmd := exec.Command(dockerBinary, "--config", "/tmp/docker-isolated", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)
}

func (s *DockerTrustSuite) TestUntrustedPull(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	dockerCmd(c, "push", repoName)
	dockerCmd(c, "rmi", repoName)

	// Try trusted pull on untrusted tag
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)
	if err == nil {
		c.Fatalf("Error expected when running trusted pull with:\n%s", out)
	}

	if !strings.Contains(string(out), "no trust data available") {
		c.Fatalf("Missing expected output on trusted pull:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestPullWhenCertExpired(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := s.setupTrustedImage(c, "trusted-cert-expired")

	// Certificates have 10 years of expiration
	elevenYearsFromNow := time.Now().Add(time.Hour * 24 * 365 * 11)

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try pull
		pullCmd := exec.Command(dockerBinary, "pull", repoName)
		s.trustedCmd(pullCmd)
		out, _, err := runCommandWithOutput(pullCmd)
		if err == nil {
			c.Fatalf("Error running trusted pull in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "could not validate the path to a trusted root") {
			c.Fatalf("Missing expected output on trusted pull in the distant future:\n%s", out)
		}
	})

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try pull
		pullCmd := exec.Command(dockerBinary, "pull", "--disable-content-trust", repoName)
		s.trustedCmd(pullCmd)
		out, _, err := runCommandWithOutput(pullCmd)
		if err != nil {
			c.Fatalf("Error running untrusted pull in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "Status: Downloaded") {
			c.Fatalf("Missing expected output on untrusted pull in the distant future:\n%s", out)
		}
	})
}

func (s *DockerTrustSuite) TestTrustedPullFromBadTrustServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclievilpull/trusted:latest", privateRegistryURL)
	evilLocalConfigDir, err := ioutil.TempDir("", "evil-local-config-dir")
	if err != nil {
		c.Fatalf("Failed to create local temp dir")
	}

	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Kill the notary server, start a new "evil" one.
	s.not.Close()
	s.not, err = newTestNotary(c)
	if err != nil {
		c.Fatalf("Restarting notary server failed.")
	}

	// In order to make an evil server, lets re-init a client (with a different trust dir) and push new data.
	// tag an image and upload it to the private registry
	dockerCmd(c, "--config", evilLocalConfigDir, "tag", "busybox", repoName)

	// Push up to the new server
	pushCmd = exec.Command(dockerBinary, "--config", evilLocalConfigDir, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Now, try pulling with the original client from this new trust server. This should fail.
	pullCmd = exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err == nil {
		c.Fatalf("Expected to fail on this pull due to different remote data: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "failed to validate data with current trusted certificates") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPullWithExpiredSnapshot(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := fmt.Sprintf("%v/dockercliexpiredtimestamppull/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Snapshots last for three years. This should be expired
	fourYearsLater := time.Now().Add(time.Hour * 24 * 365 * 4)

	// Should succeed because the server transparently re-signs one
	runAtDifferentDate(fourYearsLater, func() {
		// Try pull
		pullCmd := exec.Command(dockerBinary, "pull", repoName)
		s.trustedCmd(pullCmd)
		out, _, err = runCommandWithOutput(pullCmd)
		if err == nil {
			c.Fatalf("Missing expected error running trusted pull with expired snapshots")
		}

		if !strings.Contains(string(out), "repository out-of-date") {
			c.Fatalf("Missing expected output on trusted pull with expired snapshot:\n%s", out)
		}
	})
}

// Test that pull continues after client has disconnected. #15589
func (s *DockerTrustSuite) TestPullClientDisconnect(c *check.C) {
	testRequires(c, Network)

	repoName := "hello-world:latest"

	dockerCmdWithError(c, "rmi", repoName) // clean just in case

	pullCmd := exec.Command(dockerBinary, "pull", repoName)

	stdout, err := pullCmd.StdoutPipe()
	c.Assert(err, check.IsNil)

	err = pullCmd.Start()
	c.Assert(err, check.IsNil)

	// cancel as soon as we get some output
	buf := make([]byte, 10)
	_, err = stdout.Read(buf)
	c.Assert(err, check.IsNil)

	err = pullCmd.Process.Kill()
	c.Assert(err, check.IsNil)

	maxAttempts := 20
	for i := 0; ; i++ {
		if _, _, err := dockerCmdWithError(c, "inspect", repoName); err == nil {
			break
		}
		if i >= maxAttempts {
			c.Fatal("Timeout reached. Image was not pulled after client disconnected.")
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
