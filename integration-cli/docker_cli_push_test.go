package main

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/distribution/digest"
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

	digest1 := digest.DigestRegexp.FindString(out1)
	c.Assert(len(digest1), checker.GreaterThan, 0, check.Commentf("no digest found for pushed manifest"))

	destRepoName := fmt.Sprintf("%v/dockercli/crossrepopush", privateRegistryURL)
	// retag the image to upload the same layers to another repo in the same registry
	dockerCmd(c, "tag", "busybox", destRepoName)
	// push the image to the registry
	out2, _, err := dockerCmdWithError("push", destRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out2))
	// ensure that layers were mounted from the first repo during push
	c.Assert(strings.Contains(out2, "Mounted from dockercli/busybox"), check.Equals, true)

	digest2 := digest.DigestRegexp.FindString(out2)
	c.Assert(len(digest2), checker.GreaterThan, 0, check.Commentf("no digest found for pushed manifest"))
	c.Assert(digest1, check.Equals, digest2)

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

	digest1 := digest.DigestRegexp.FindString(out1)
	c.Assert(len(digest1), checker.GreaterThan, 0, check.Commentf("no digest found for pushed manifest"))

	destRepoName := fmt.Sprintf("%v/dockercli/crossrepopush", privateRegistryURL)
	// retag the image to upload the same layers to another repo in the same registry
	dockerCmd(c, "tag", "busybox", destRepoName)
	// push the image to the registry
	out2, _, err := dockerCmdWithError("push", destRepoName)
	c.Assert(err, check.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out2))
	// schema1 registry should not support cross-repo layer mounts, so ensure that this does not happen
	c.Assert(strings.Contains(out2, "Mounted from dockercli/busybox"), check.Equals, false)

	digest2 := digest.DigestRegexp.FindString(out2)
	c.Assert(len(digest2), checker.GreaterThan, 0, check.Commentf("no digest found for pushed manifest"))
	c.Assert(digest1, check.Equals, digest2)

	// ensure that we can pull and run the second pushed repository
	dockerCmd(c, "rmi", destRepoName)
	dockerCmd(c, "pull", destRepoName)
	out3, _ := dockerCmd(c, "run", destRepoName, "echo", "-n", "hello world")
	c.Assert(out3, check.Equals, "hello world")
}

func (s *DockerTrustSuite) TestTrustedPush(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclitrusted/pushtest:latest", privateRegistryURL)
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
	repoName := fmt.Sprintf("%v/dockerclitrusted/failingserver:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com:81/")
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.NotNil, check.Commentf("Missing error while running trusted push w/ no server"))
	c.Assert(out, checker.Contains, "error contacting notary server", check.Commentf("Missing expected output on trusted push"))
}

func (s *DockerTrustSuite) TestTrustedPushWithoutServerAndUntrusted(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclitrusted/trustedandnot:latest", privateRegistryURL)
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
