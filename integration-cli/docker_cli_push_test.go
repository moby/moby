package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/image"
	"github.com/go-check/check"
)

// Pushing an image to a private registry.
func (s *DockerRegistrySuite) TestPushBusyboxImage(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	// push the image to the registry
	dockerCmd(c, "push", repoName)
}

// pushing an image without a prefix should throw an error
func (s *DockerSuite) TestPushUnprefixedRepo(c *check.C) {
	if out, _, err := dockerCmdWithError("push", "busybox"); err == nil {
		c.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
}

func (s *DockerRegistrySuite) TestPushUntagged(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	expected := "Repository does not exist"
	if out, _, err := dockerCmdWithError("push", repoName); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: output %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerRegistrySuite) TestPushBadTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox:latest", privateRegistryURL)

	expected := "does not exist"

	if out, _, err := dockerCmdWithError("push", repoName); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: output %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerRegistrySuite) TestPushMultipleTags(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	repoTag1 := fmt.Sprintf("%v/dockercli/busybox:t1", privateRegistryURL)
	repoTag2 := fmt.Sprintf("%v/dockercli/busybox:t2", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoTag1)

	dockerCmd(c, "tag", "busybox", repoTag2)

	dockerCmd(c, "push", repoName)

	// Ensure layer list is equivalent for repoTag1 and repoTag2
	out1, _ := dockerCmd(c, "pull", repoTag1)
	if strings.Contains(out1, "Tag t1 not found") {
		c.Fatalf("Unable to pull pushed image: %s", out1)
	}
	imageAlreadyExists := ": Image already exists"
	var out1Lines []string
	for _, outputLine := range strings.Split(out1, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	out2, _ := dockerCmd(c, "pull", repoTag2)
	if strings.Contains(out2, "Tag t2 not found") {
		c.Fatalf("Unable to pull pushed image: %s", out1)
	}
	var out2Lines []string
	for _, outputLine := range strings.Split(out2, "\n") {
		if strings.Contains(outputLine, imageAlreadyExists) {
			out1Lines = append(out1Lines, outputLine)
		}
	}

	if len(out1Lines) != len(out2Lines) {
		c.Fatalf("Mismatched output length:\n%s\n%s", out1, out2)
	}

	for i := range out1Lines {
		if out1Lines[i] != out2Lines[i] {
			c.Fatalf("Mismatched output line:\n%s\n%s", out1Lines[i], out2Lines[i])
		}
	}
}

// TestPushBadParentChain tries to push an image with a corrupted parent chain
// in the v1compatibility files, and makes sure the push process fixes it.
func (s *DockerRegistrySuite) TestPushBadParentChain(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/badparent", privateRegistryURL)

	id, err := buildImage(repoName, `
	    FROM busybox
	    CMD echo "adding another layer"
	    `, true)
	if err != nil {
		c.Fatal(err)
	}

	// Push to create v1compatibility file
	dockerCmd(c, "push", repoName)

	// Corrupt the parent in the v1compatibility file from the top layer
	filename := filepath.Join(dockerBasePath, "graph", id, "v1Compatibility")

	jsonBytes, err := ioutil.ReadFile(filename)
	c.Assert(err, check.IsNil, check.Commentf("Could not read v1Compatibility file: %s", err))

	var img image.Image
	err = json.Unmarshal(jsonBytes, &img)
	c.Assert(err, check.IsNil, check.Commentf("Could not unmarshal json: %s", err))

	img.Parent = "1234123412341234123412341234123412341234123412341234123412341234"

	jsonBytes, err = json.Marshal(&img)
	c.Assert(err, check.IsNil, check.Commentf("Could not marshal json: %s", err))

	err = ioutil.WriteFile(filename, jsonBytes, 0600)
	c.Assert(err, check.IsNil, check.Commentf("Could not write v1Compatibility file: %s", err))

	dockerCmd(c, "push", repoName)

	// pull should succeed
	dockerCmd(c, "pull", repoName)
}

func (s *DockerRegistrySuite) TestPushEmptyLayer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/emptylayer", privateRegistryURL)
	emptyTarball, err := ioutil.TempFile("", "empty_tarball")
	if err != nil {
		c.Fatalf("Unable to create test file: %v", err)
	}
	tw := tar.NewWriter(emptyTarball)
	err = tw.Close()
	if err != nil {
		c.Fatalf("Error creating empty tarball: %v", err)
	}
	freader, err := os.Open(emptyTarball.Name())
	if err != nil {
		c.Fatalf("Could not open test tarball: %v", err)
	}

	importCmd := exec.Command(dockerBinary, "import", "-", repoName)
	importCmd.Stdin = freader
	out, _, err := runCommandWithOutput(importCmd)
	if err != nil {
		c.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	// Now verify we can push it
	if out, _, err := dockerCmdWithError("push", repoName); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
}

func (s *DockerTrustSuite) TestTrustedPush(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
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
}

func (s *DockerTrustSuite) TestTrustedPushWithEnvPasswords(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "12345678")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
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
	if err != nil {
		c.Fatalf("Error running trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithFaillingServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com:81/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Missing error while running trusted push w/ no server")
	}

	if !strings.Contains(string(out), "error contacting notary server") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithoutServerAndUntrusted(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", "--disable-content-trust", repoName)
	s.trustedCmdWithServer(pushCmd, "https://example.com/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push with no server and --disable-content-trust failed: %s\n%s", err, out)
	}

	if strings.Contains(string(out), "Error establishing connection to notary repository") {
		c.Fatalf("Missing expected output on trusted push with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	dockerCmd(c, "push", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithExistingSignedTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclipushpush/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Do a trusted push
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}

	// Do another trusted push
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push with existing tag:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try pull to ensure the double push did not break our ability to pull
	pullCmd := exec.Command(dockerBinary, "pull", repoName)
	s.trustedCmd(pullCmd)
	out, _, err = runCommandWithOutput(pullCmd)
	if err != nil {
		c.Fatalf("Error running trusted pull: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Status: Downloaded") {
		c.Fatalf("Missing expected output on trusted pull with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithIncorrectPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercliincorretpwd/trusted:latest", privateRegistryURL)
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

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short targets passphrase: \n%s", out)
	}

	if !strings.Contains(string(out), "password invalid, operation has failed") {
		c.Fatalf("Missing expected output on trusted push with short targets/snapsnot passphrase:\n%s", out)
	}
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
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Push with wrong passphrases
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithDeprecatedEnvPassphrases(pushCmd, "12345678", "87654321")
	out, _, err = runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short targets passphrase: \n%s", out)
	}

	if !strings.Contains(string(out), "password invalid, operation has failed") {
		c.Fatalf("Missing expected output on trusted push with short targets/snapsnot passphrase:\n%s", out)
	}
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
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Snapshots last for three years. This should be expired
	fourYearsLater := time.Now().Add(time.Hour * 24 * 365 * 4)

	runAtDifferentDate(fourYearsLater, func() {
		// Push with wrong passphrases
		pushCmd = exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err = runCommandWithOutput(pushCmd)
		if err == nil {
			c.Fatalf("Error missing from trusted push with expired snapshot: \n%s", out)
		}

		if !strings.Contains(string(out), "repository out-of-date") {
			c.Fatalf("Missing expected output on trusted push with expired snapshot:\n%s", out)
		}
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
	if err != nil {
		c.Fatalf("trusted push failed: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// The timestamps expire in two weeks. Lets check three
	threeWeeksLater := time.Now().Add(time.Hour * 24 * 21)

	// Should succeed because the server transparently re-signs one
	runAtDifferentDate(threeWeeksLater, func() {
		pushCmd := exec.Command(dockerBinary, "push", repoName)
		s.trustedCmd(pushCmd)
		out, _, err := runCommandWithOutput(pushCmd)
		if err != nil {
			c.Fatalf("Error running trusted push: %s\n%s", err, out)
		}
		if !strings.Contains(string(out), "Signing and pushing trust metadata") {
			c.Fatalf("Missing expected output on trusted push with expired timestamp:\n%s", out)
		}
	})
}
