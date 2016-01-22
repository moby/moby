package main

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

// Pushing an image to a private registry.
func testPushBusyboxImage(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	// push the image to the registry
	dockerCmd(c, "push", repoName)
}

func (s *DockerRegistrySuite) TestPushBusyboxImage(c *check.C) {
	testPushBusyboxImage(c)
}

func (s *DockerSchema1RegistrySuite) TestPushBusyboxImage(c *check.C) {
	testPushBusyboxImage(c)
}

// pushing an image without a prefix should throw an error
func (s *DockerSuite) TestPushUnprefixedRepo(c *check.C) {
	out, _, err := dockerCmdWithError("push", "busybox")
	c.Assert(err, check.NotNil, check.Commentf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out))
}

func testPushUntagged(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	expected := "Repository does not exist"

	out, _, err := dockerCmdWithError("push", repoName)
	c.Assert(err, check.NotNil, check.Commentf("pushing the image to the private registry should have failed: output %q", out))
	c.Assert(out, checker.Contains, expected, check.Commentf("pushing the image failed"))
}

func (s *DockerRegistrySuite) TestPushUntagged(c *check.C) {
	testPushUntagged(c)
}

func (s *DockerSchema1RegistrySuite) TestPushUntagged(c *check.C) {
	testPushUntagged(c)
}

func testPushBadTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox:latest", privateRegistryURL)
	expected := "does not exist"

	out, _, err := dockerCmdWithError("push", repoName)
	c.Assert(err, check.NotNil, check.Commentf("pushing the image to the private registry should have failed: output %q", out))
	c.Assert(out, checker.Contains, expected, check.Commentf("pushing the image failed"))
}

func (s *DockerRegistrySuite) TestPushBadTag(c *check.C) {
	testPushBadTag(c)
}

func (s *DockerSchema1RegistrySuite) TestPushBadTag(c *check.C) {
	testPushBadTag(c)
}

func testPushMultipleTags(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	repoTag1 := fmt.Sprintf("%v/dockercli/busybox:t1", privateRegistryURL)
	repoTag2 := fmt.Sprintf("%v/dockercli/busybox:t2", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoTag1)

	dockerCmd(c, "tag", "busybox", repoTag2)

	dockerCmd(c, "push", repoName)

	// Ensure layer list is equivalent for repoTag1 and repoTag2
	out1, _ := dockerCmd(c, "pull", repoTag1)

	imageAlreadyExists := ": Image already exists"
	var out1Lines []string
	for _, outputLine := range strings.Split(out1, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	out2, _ := dockerCmd(c, "pull", repoTag2)

	var out2Lines []string
	for _, outputLine := range strings.Split(out2, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}
	c.Assert(out2Lines, checker.HasLen, len(out1Lines))

	for i := range out1Lines {
		c.Assert(out1Lines[i], checker.Equals, out2Lines[i])
	}
}

func (s *DockerRegistrySuite) TestPushMultipleTags(c *check.C) {
	testPushMultipleTags(c)
}

func (s *DockerSchema1RegistrySuite) TestPushMultipleTags(c *check.C) {
	testPushMultipleTags(c)
}

func testPushEmptyLayer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/emptylayer", privateRegistryURL)
	emptyTarball, err := ioutil.TempFile("", "empty_tarball")
	c.Assert(err, check.IsNil, check.Commentf("Unable to create test file"))

	tw := tar.NewWriter(emptyTarball)
	err = tw.Close()
	c.Assert(err, check.IsNil, check.Commentf("Error creating empty tarball"))

	freader, err := os.Open(emptyTarball.Name())
	c.Assert(err, check.IsNil, check.Commentf("Could not open test tarball"))

	importCmd := exec.Command(dockerBinary, "import", "-", repoName)
	importCmd.Stdin = freader
	out, _, err := runCommandWithOutput(importCmd)
	c.Assert(err, check.IsNil, check.Commentf("import failed: %q", out))

	// Now verify we can push it
	out, _, err = dockerCmdWithError("push", repoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out))
}

func (s *DockerRegistrySuite) TestPushEmptyLayer(c *check.C) {
	testPushEmptyLayer(c)
}

func (s *DockerSchema1RegistrySuite) TestPushEmptyLayer(c *check.C) {
	testPushEmptyLayer(c)
}

func (s *DockerRegistrySuite) TestCrossRepositoryLayerPush(c *check.C) {
	sourceRepoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	dockerCmd(c, "tag", "busybox", sourceRepoName)
	// push the image to the registry
	out1, _, err := dockerCmdWithError("push", sourceRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out1))
	// ensure that none of the layers were mounted from another repository during push
	c.Assert(strings.Contains(out1, "Mounted from"), check.Equals, false)

	destRepoName := fmt.Sprintf("%v/dockercli/crossrepopush", privateRegistryURL)
	// retag the image to upload the same layers to another repo in the same registry
	dockerCmd(c, "tag", "busybox", destRepoName)
	// push the image to the registry
	out2, _, err := dockerCmdWithError("push", destRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out2))
	// ensure that layers were mounted from the first repo during push
	c.Assert(strings.Contains(out2, "Mounted from dockercli/busybox"), check.Equals, true)

	// ensure that we can pull and run the cross-repo-pushed repository
	dockerCmd(c, "rmi", destRepoName)
	dockerCmd(c, "pull", destRepoName)
	out3, _ := dockerCmd(c, "run", destRepoName, "echo", "-n", "hello world")
	c.Assert(out3, check.Equals, "hello world")
}

func (s *DockerSchema1RegistrySuite) TestCrossRepositoryLayerPushNotSupported(c *check.C) {
	sourceRepoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	dockerCmd(c, "tag", "busybox", sourceRepoName)
	// push the image to the registry
	out1, _, err := dockerCmdWithError("push", sourceRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out1))
	// ensure that none of the layers were mounted from another repository during push
	c.Assert(strings.Contains(out1, "Mounted from"), check.Equals, false)

	destRepoName := fmt.Sprintf("%v/dockercli/crossrepopush", privateRegistryURL)
	// retag the image to upload the same layers to another repo in the same registry
	dockerCmd(c, "tag", "busybox", destRepoName)
	// push the image to the registry
	out2, _, err := dockerCmdWithError("push", destRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out2))
	// schema1 registry should not support cross-repo layer mounts, so ensure that this does not happen
	c.Assert(strings.Contains(out2, "Mounted from dockercli/busybox"), check.Equals, false)

	// ensure that we can pull and run the second pushed repository
	dockerCmd(c, "rmi", destRepoName)
	dockerCmd(c, "pull", destRepoName)
	out3, _ := dockerCmd(c, "run", destRepoName, "echo", "-n", "hello world")
	c.Assert(out3, check.Equals, "hello world")
}

func (s *DockerTrustSuite) TestTrustedPush(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("Error running trusted push: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))

	// Try pull after push
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))
}

func (s *DockerTrustSuite) TestTrustedPushWithEnvPasswords(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclienv/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "12345678")
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("Error running trusted push: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))

	// Try pull after push
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))
}

// This test ensures backwards compatibility with old ENV variables. Should be
// deprecated by 1.10
func (s *DockerTrustSuite) TestTrustedPushWithDeprecatedEnvPasswords(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusteddeprecated:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithDeprecatedEnvPassphrases(pushCmd, "12345678", "12345678")
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("Error running trusted push: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))
}

func (s *DockerTrustSuite) TestTrustedPushWithFailingServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com:81/")
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.NotNil, check.Commentf("Missing error while running trusted push w/ no server"))
	c.Assert(out, checker.Contains, "error contacting notary server", check.Commentf("Missing expected output on trusted push"))
}

func (s *DockerTrustSuite) TestTrustedPushWithoutServerAndUntrusted(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", "--disable-content-trust", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com/")
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push with no server and --disable-content-trust failed: %s\n%s", err, out))
	c.Assert(out, check.Not(checker.Contains), "Error establishing connection to notary repository", check.Commentf("Missing expected output on trusted push with --disable-content-trust:"))
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclitag/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	dockerCmd(c, "push", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push with existing tag"))

	// Try pull after push
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingSignedTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclipushpush/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Do a trusted push
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push with existing tag"))

	// Do another trusted push
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push with existing tag"))

	dockerCmd(c, "rmi", repoName)

	// Try pull to ensure the double push did not break our ability to pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf("Error running trusted pull: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Status: Downloaded", check.Commentf("Missing expected output on trusted pull with --disable-content-trust"))

}

func (s *DockerTrustSuite) TestTrustedPushWithIncorrectPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercliincorretpwd/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push:\n%s", out))

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	c.Assert(err, check.NotNil, check.Commentf("Error missing from trusted push with short targets passphrase: \n%s", out))
	c.Assert(out, checker.Contains, "could not find necessary signing keys", check.Commentf("Missing expected output on trusted push with short targets/snapsnot passphrase"))
}

// This test ensures backwards compatibility with old ENV variables. Should be
// deprecated by 1.10
func (s *DockerTrustSuite) TestTrustedPushWithIncorrectDeprecatedPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercliincorretdeprecatedpwd/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithDeprecatedEnvPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	c.Assert(err, check.NotNil, check.Commentf("Error missing from trusted push with short targets passphrase: \n%s", out))
	c.Assert(out, checker.Contains, "could not find necessary signing keys", check.Commentf("Missing expected output on trusted push with short targets/snapsnot passphrase"))
}

func (s *DockerTrustSuite) TestTrustedPushWithExpiredSnapshot(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := fmt.Sprintf("%v/dockercliexpiredsnapshot/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))

	// Snapshots last for three years. This should be expired
	fourYearsLater := time.Now().Add(time.Hour * 24 * 365 * 4)

	runAtDifferentDate(fourYearsLater, func() {
		// Push with wrong passphrases
		pushCmd = exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err = runCommandWithOutput(pushCmd)
		c.Assert(err, check.NotNil, check.Commentf("Error missing from trusted push with expired snapshot: \n%s", out))
		c.Assert(out, checker.Contains, "repository out-of-date", check.Commentf("Missing expected output on trusted push with expired snapshot"))
	})
}

func (s *DockerTrustSuite) TestTrustedPushWithExpiredTimestamp(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := fmt.Sprintf("%v/dockercliexpiredtimestamppush/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrases
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push"))

	// The timestamps expire in two weeks. Lets check three
	threeWeeksLater := time.Now().Add(time.Hour * 24 * 21)

	// Should succeed because the server transparently re-signs one
	runAtDifferentDate(threeWeeksLater, func() {
		pushCmd := exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err := runCommandWithOutput(pushCmd)
		c.Assert(err, check.IsNil, check.Commentf("Error running trusted push: %s\n%s", err, out))
		c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push with expired timestamp"))
	})
}

func (s *DockerTrustSuite) TestTrustedPushWithReleasesDelegation(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclireleasedelegation/trusted", privateRegistryURL)
	targetName := fmt.Sprintf("%s:latest", repoName)
	pwd := "12345678"
	s.setupDelegations(c, repoName, pwd)

	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", targetName)

	pushCmd := exec.Command(dockerBinary, "-D", "push", targetName)
	s.trustedCmdWithPassphrases(pushCmd, pwd, pwd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("trusted push failed: %s\n%s", err, out))
	c.Assert(out, checker.Contains, "Signing and pushing trust metadata", check.Commentf("Missing expected output on trusted push with existing tag"))

	// Try pull after push
	pullCmd := exec.Command(dockerBinary, "pull", targetName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))

	// check to make sure that the target has been added to targets/releases and not targets
	contents, err := ioutil.ReadFile(filepath.Join(cliconfig.ConfigDir(), "trust/tuf", repoName, "metadata/targets.json"))
	c.Assert(err, check.IsNil, check.Commentf("Unable to read targets metadata"))
	c.Assert(strings.Contains(string(contents), `"latest"`), checker.False, check.Commentf(string(contents)))

	contents, err = ioutil.ReadFile(filepath.Join(cliconfig.ConfigDir(), "trust/tuf", repoName, "metadata/targets/releases.json"))
	c.Assert(err, check.IsNil, check.Commentf("Unable to read targets/releases metadata"))
	c.Assert(string(contents), checker.Contains, `"latest"`, check.Commentf(string(contents)))
}

func (s *DockerSuite) TestPushOfficialImage(c *check.C) {
	var reErr = regexp.MustCompile(`rename your repository to[^:]*:\s*docker\.io/<user>/busybox\b`)

	// push busybox to public registry as "library/busybox"
	cmd := exec.Command(dockerBinary, "push", "library/busybox")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.Fatalf("Failed to get stdout pipe for process: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.Fatalf("Failed to get stderr pipe for process: %v", err)
	}
	if err := cmd.Start(); err != nil {
		c.Fatalf("Failed to start pushing to public registry: %v", err)
	}
	outReader := bufio.NewReader(stdout)
	errReader := bufio.NewReader(stderr)
	line, isPrefix, err := errReader.ReadLine()
	if err != nil {
		c.Fatalf("Failed to read farewell: %v", err)
	}
	if isPrefix {
		c.Errorf("Got unexpectedly long output.")
	}
	if !reErr.Match(line) {
		c.Errorf("Got unexpected output %q", line)
	}
	if line, _, err = outReader.ReadLine(); err != io.EOF {
		c.Errorf("Expected EOF, not: %q", line)
	}
	for ; err != io.EOF; line, _, err = errReader.ReadLine() {
		c.Errorf("Expected no message on stderr, got: %q", string(line))
	}

	// Wait for command to finish with short timeout.
	finish := make(chan struct{})
	go func() {
		if err := cmd.Wait(); err == nil {
			c.Error("Push command should have failed.")
		}
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(1 * time.Second):
		c.Fatalf("Docker push failed to exit.")
	}
}

func (s *DockerRegistrySuite) TestPushToAdditionalRegistry(c *check.C) {
	if err := s.d.StartWithBusybox("--add-registry=" + s.reg.url); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", s.reg.url, err)
	}

	bbImg := s.d.getAndTestImageEntry(c, 1, "busybox", "")

	// push busybox to additional registry as "library/busybox" and remove all local images
	if out, err := s.d.Cmd("tag", "busybox", "library/busybox"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", "library/busybox"); err != nil {
		c.Fatalf("failed to push image library/busybox: error %v, output %q", err, out)
	}
	toRemove := []string{"busybox", "library/busybox"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 0, "", "")

	// pull it from additional registry
	if _, err := s.d.Cmd("pull", "library/busybox"); err != nil {
		c.Fatalf("we should have been able to pull library/busybox from %q: %v", s.reg.url, err)
	}
	bb2Img := s.d.getAndTestImageEntry(c, 1, s.reg.url+"/library/busybox", "")
	if bb2Img.size != bbImg.size {
		c.Fatalf("expected %s and %s to have the same size (%s != %s)", bb2Img.name, bbImg.name, bb2Img.size, bbImg.size)
	}
}

func (s *DockerRegistrySuite) TestPushCustomTagToAdditionalRegistry(c *check.C) {
	if err := s.d.StartWithBusybox("--add-registry=" + s.reg.url); err != nil {
		c.Fatalf("we should have been able to start the daemon with passing add-registry=%s: %v", s.reg.url, err)
	}

	busyboxID := s.d.getAndTestImageEntry(c, 1, "busybox", "").id

	if out, err := s.d.Cmd("tag", "busybox", "user/busybox:1.2.3"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("tag", "busybox", s.reg.url+"/user/busybox:latest"); err != nil {
		c.Fatalf("failed to tag image %s: error %v, output %q", "busybox", err, out)
	}
	if out, err := s.d.Cmd("push", "user/busybox:1.2.3"); err != nil {
		c.Fatalf("failed to push image user/busybox: error %v, output %q", err, out)
	}
	s.d.getAndTestImageEntry(c, 3, "user/busybox", busyboxID)
	toRemove := []string{"user/busybox:1.2.3"}
	if out, err := s.d.Cmd("rmi", toRemove...); err != nil {
		c.Fatalf("failed to remove images %v: %v, output: %s", toRemove, err, out)
	}
	s.d.getAndTestImageEntry(c, 2, s.reg.url+"/user/busybox", busyboxID)
}

func (s *DockerRegistriesSuite) TestPushNeedsAuth(c *check.C) {
	c.Assert(s.d.StartWithBusybox("--add-registry="+s.regWithAuth.url), check.IsNil)

	repo := fmt.Sprintf("%s/runcom/busybox", s.regWithAuth.url)
	repoUnqualified := "runcom/busybox"

	out, err := s.d.Cmd("tag", "busybox", repoUnqualified)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// this means it needs auth...
	resp, err := http.Get(fmt.Sprintf("http://%s/v2/", s.regWithAuth.url))
	c.Assert(err, check.IsNil)
	c.Assert(resp.StatusCode, check.Equals, http.StatusUnauthorized)

	// login with the registry...
	out, err = s.d.Cmd("login", "-u", s.regWithAuth.username, "-p", s.regWithAuth.password, "-e", s.regWithAuth.email, s.regWithAuth.url)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// push to private registry with unqualified image name
	out, err = s.d.Cmd("push", repoUnqualified)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// remove the repo locally
	out, err = s.d.Cmd("rmi", "-f", repoUnqualified)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// pull the image from the private registry so that we're sure it was pushed there
	out, err = s.d.Cmd("pull", repo)
	c.Assert(err, check.IsNil, check.Commentf(out))
	expected := fmt.Sprintf("Pulling from %s", repo)
	if !strings.Contains(out, expected) {
		c.Fatalf("Wanted %s, got %s", expected, out)
	}
}
