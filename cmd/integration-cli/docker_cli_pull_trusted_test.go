package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
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
	repoName := fmt.Sprintf("%v/dockercliuntrusted/pulltest:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	dockerCmd(c, "push", repoName)
	dockerCmd(c, "rmi", repoName)

	// Try trusted pull on untrusted tag
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err := runCommandWithOutput(pullCmd)

	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(string(out), checker.Contains, "Error: remote trust data does not exist", check.Commentf(out))
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

	// Now, try pulling with the original client from this new trust server. This should fail because the new root is invalid.
	pullCmd = exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err == nil {
		c.Fatalf("Continuing with cached data even though it's an invalid root rotation: %s\n%s", err, out)
	}
	if !strings.Contains(out, "could not rotate trust to a new trusted root") {
		c.Fatalf("Missing expected output on trusted pull:\n%s", out)
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

func (s *DockerTrustSuite) TestTrustedPullDelete(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/%s:latest", privateRegistryURL, "trusted-pull-delete")
	// tag the image and upload it to the private registry
	_, err := buildImage(repoName, `
                    FROM busybox
                    CMD echo trustedpulldelete
                `, true)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	if out, status := dockerCmd(c, "rmi", repoName); status != 0 {
		c.Fatalf("Error removing image %q\n%s", repoName, out)
	}

	// Try pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)

	c.Assert(err, check.IsNil, check.Commentf(out))

	matches := digestRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	imageID := inspectField(c, repoName, "Id")

	imageByDigest := repoName + "@" + pullDigest
	byDigestID := inspectField(c, imageByDigest, "Id")

	c.Assert(byDigestID, checker.Equals, imageID)

	// rmi of tag should also remove the digest reference
	dockerCmd(c, "rmi", repoName)

	_, err = inspectFieldWithError(imageByDigest, "Id")
	c.Assert(err, checker.NotNil, check.Commentf("digest reference should have been removed"))

	_, err = inspectFieldWithError(imageID, "Id")
	c.Assert(err, checker.NotNil, check.Commentf("image should have been deleted"))
}

func (s *DockerTrustSuite) TestTrustedPullReadsFromReleasesRole(c *check.C) {
	testRequires(c, NotaryHosting)
	repoName := fmt.Sprintf("%v/dockerclireleasesdelegationpulling/trusted", privateRegistryURL)
	targetName := fmt.Sprintf("%s:latest", repoName)

	// Push with targets first, initializing the repo
	dockerCmd(c, "tag", "busybox", targetName)
	pushCmd := exec.Command(dockerBinary, "push", targetName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	s.assertTargetInRoles(c, repoName, "latest", "targets")

	// Try pull, check we retrieve from targets role
	pullCmd := exec.Command(dockerBinary, "-D", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "retrieving target for targets role")

	// Now we'll create the releases role, and try pushing and pulling
	s.notaryCreateDelegation(c, repoName, "targets/releases", s.not.keys[0].Public)
	s.notaryImportKey(c, repoName, "targets/releases", s.not.keys[0].Private)
	s.notaryPublish(c, repoName)

	// try a pull, check that we can still pull because we can still read the
	// old tag in the targets role
	pullCmd = exec.Command(dockerBinary, "-D", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "retrieving target for targets role")

	// try a pull -a, check that it succeeds because we can still pull from the
	// targets role
	pullCmd = exec.Command(dockerBinary, "-D", "pull", "-a", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))

	// Push, should sign with targets/releases
	dockerCmd(c, "tag", "busybox", targetName)
	pushCmd = exec.Command(dockerBinary, "push", targetName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	s.assertTargetInRoles(c, repoName, "latest", "targets", "targets/releases")

	// Try pull, check we retrieve from targets/releases role
	pullCmd = exec.Command(dockerBinary, "-D", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(out, checker.Contains, "retrieving target for targets/releases role")

	// Create another delegation that we'll sign with
	s.notaryCreateDelegation(c, repoName, "targets/other", s.not.keys[1].Public)
	s.notaryImportKey(c, repoName, "targets/other", s.not.keys[1].Private)
	s.notaryPublish(c, repoName)

	dockerCmd(c, "tag", "busybox", targetName)
	pushCmd = exec.Command(dockerBinary, "push", targetName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	s.assertTargetInRoles(c, repoName, "latest", "targets", "targets/releases", "targets/other")

	// Try pull, check we retrieve from targets/releases role
	pullCmd = exec.Command(dockerBinary, "-D", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(out, checker.Contains, "retrieving target for targets/releases role")
}

func (s *DockerTrustSuite) TestTrustedPullIgnoresOtherDelegationRoles(c *check.C) {
	testRequires(c, NotaryHosting)
	repoName := fmt.Sprintf("%v/dockerclipullotherdelegation/trusted", privateRegistryURL)
	targetName := fmt.Sprintf("%s:latest", repoName)

	// We'll create a repo first with a non-release delegation role, so that when we
	// push we'll sign it into the delegation role
	s.notaryInitRepo(c, repoName)
	s.notaryCreateDelegation(c, repoName, "targets/other", s.not.keys[0].Public)
	s.notaryImportKey(c, repoName, "targets/other", s.not.keys[0].Private)
	s.notaryPublish(c, repoName)

	// Push should write to the delegation role, not targets
	dockerCmd(c, "tag", "busybox", targetName)
	pushCmd := exec.Command(dockerBinary, "push", targetName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
	s.assertTargetInRoles(c, repoName, "latest", "targets/other")
	s.assertTargetNotInRoles(c, repoName, "latest", "targets")

	// Try pull - we should fail, since pull will only pull from the targets/releases
	// role or the targets role
	pullCmd := exec.Command(dockerBinary, "-D", "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "No trust data for")

	// try a pull -a: we should fail since pull will only pull from the targets/releases
	// role or the targets role
	pullCmd = exec.Command(dockerBinary, "-D", "pull", "-a", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	c.Assert(err, check.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, "No trusted tags for")
}
