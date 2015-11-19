package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerTrustSuite) TestTrustedPull(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-pull")

	// Try pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf(out))

	dockerCmd(c, "rmi", repoName)
	// Try untrusted pull to ensure we pushed the tag to the registry
	pullCmd = exec.Command(dockerBinary, "pull", "--disable-content-trust=true", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))

}

func (s *DockerTrustSuite) TestTrustedIsolatedPull(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-isolated-pull")

	// Try pull (run from isolated directory without trust information)
	pullCmd := exec.Command(dockerBinary, "--config", "/tmp/docker-isolated", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf(string(out)))

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

	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "no trust data available", check.Commentf(out))
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

		c.Assert(err, check.NotNil, check.Commentf(out))
		c.Assert(string(out), checker.Contains, "could not validate the path to a trusted root", check.Commentf(out))
	})

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try pull
		pullCmd := exec.Command(dockerBinary, "pull", "--disable-content-trust", repoName)
		s.trustedCmd(pullCmd)
		out, _, err := runCommandWithOutput(pullCmd)

		c.Assert(err, check.IsNil, check.Commentf(out))
		c.Assert(string(out), checker.Contains, "Status: Downloaded", check.Commentf(out))
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

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Signing and pushing trust metadata", check.Commentf(out))
	dockerCmd(c, "rmi", repoName)

	// Try pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf(out))
	dockerCmd(c, "rmi", repoName)

	// Kill the notary server, start a new "evil" one.
	s.not.Close()
	s.not, err = newTestNotary(c)

	c.Assert(err, check.IsNil, check.Commentf("Restarting notary server failed."))

	// In order to make an evil server, lets re-init a client (with a different trust dir) and push new data.
	// tag an image and upload it to the private registry
	dockerCmd(c, "--config", evilLocalConfigDir, "tag", "busybox", repoName)

	// Push up to the new server
	pushCmd = exec.Command(dockerBinary, "--config", evilLocalConfigDir, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Signing and pushing trust metadata", check.Commentf(out))

	// Now, try pulling with the original client from this new trust server. This should fail.
	pullCmd = exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)

	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "failed to validate data with current trusted certificates", check.Commentf(out))
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

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Signing and pushing trust metadata", check.Commentf(out))

	dockerCmd(c, "rmi", repoName)

	// Snapshots last for three years. This should be expired
	fourYearsLater := time.Now().Add(time.Hour * 24 * 365 * 4)

	runAtDifferentDate(fourYearsLater, func() {
		// Try pull
		pullCmd := exec.Command(dockerBinary, "pull", repoName)
		s.trustedCmd(pullCmd)
		out, _, err = runCommandWithOutput(pullCmd)

		c.Assert(err, check.NotNil, check.Commentf("Missing expected error running trusted pull with expired snapshots"))
		c.Assert(string(out), checker.Contains, "repository out-of-date", check.Commentf(out))
	})
}

func (s *DockerTrustSuite) TestTrustedOfflinePull(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-offline-pull")

	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmdWithServer(pullCmd, "https://invalidnotaryserver")
	out, _, err := runCommandWithOutput(pullCmd)

	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "error contacting notary server", check.Commentf(out))
	// Do valid trusted pull to warm cache
	pullCmd = exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf(out))

	dockerCmd(c, "rmi", repoName)

	// Try pull again with invalid notary server, should use cache
	pullCmd = exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmdWithServer(pullCmd, "https://invalidnotaryserver")
	out, _, err = runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Tagging", check.Commentf(out))
}
