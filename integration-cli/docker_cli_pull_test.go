package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"io/ioutil"

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
		if _, _, err := dockerCmdWithError("inspect", repo); err == nil {
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
	if out, exitCode, err := dockerCmdWithError("pull", verifiedName); err != nil || !strings.Contains(out, expected) {
		if err != nil || exitCode != 0 {
			c.Skip(fmt.Sprintf("pulling the '%s' image from the registry has failed: %v", verifiedName, err))
		}
		c.Fatalf("pulling a verified image failed. expected: %s\ngot: %s, %v", expected, out, err)
	}

	// pull it again
	if out, exitCode, err := dockerCmdWithError("pull", verifiedName); err != nil || strings.Contains(out, expected) {
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
	out, _, err := dockerCmdWithError("pull", name)

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
		out, exitCode, err := dockerCmdWithError("pull", name)
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

	out, exitCode, err := dockerCmdWithError("pull", "scratch")
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
func (s *DockerSuite) TestPullClientDisconnect(c *check.C) {
	testRequires(c, Network)

	repoName := "hello-world:latest"

	dockerCmdWithError("rmi", repoName) // clean just in case

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
		if _, _, err := dockerCmdWithError("inspect", repoName); err == nil {
			break
		}
		if i >= maxAttempts {
			c.Fatal("Timeout reached. Image was not pulled after client disconnected.")
		}
		time.Sleep(500 * time.Millisecond)
	}

}
