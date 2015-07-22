package main

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

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
	if out, _, err := dockerCmdWithError(c, "push", "busybox"); err == nil {
		c.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
}

func (s *DockerRegistrySuite) TestPushUntagged(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	expected := "Repository does not exist"
	if out, _, err := dockerCmdWithError(c, "push", repoName); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: output %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerRegistrySuite) TestPushBadTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox:latest", privateRegistryURL)

	expected := "does not exist"

	if out, _, err := dockerCmdWithError(c, "push", repoName); err == nil {
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

	out, _ := dockerCmd(c, "push", repoName)

	// There should be no duplicate hashes in the output
	imageSuccessfullyPushed := ": Image successfully pushed"
	imageAlreadyExists := ": Image already exists"
	imagePushHashes := make(map[string]struct{})
	outputLines := strings.Split(out, "\n")
	for _, outputLine := range outputLines {
		if strings.Contains(outputLine, imageSuccessfullyPushed) {
			hash := strings.TrimSuffix(outputLine, imageSuccessfullyPushed)
			if _, present := imagePushHashes[hash]; present {
				c.Fatalf("Duplicate image push: %s", outputLine)
			}
			imagePushHashes[hash] = struct{}{}
		} else if strings.Contains(outputLine, imageAlreadyExists) {
			hash := strings.TrimSuffix(outputLine, imageAlreadyExists)
			if _, present := imagePushHashes[hash]; present {
				c.Fatalf("Duplicate image push: %s", outputLine)
			}
			imagePushHashes[hash] = struct{}{}
		}
	}

	if len(imagePushHashes) == 0 {
		c.Fatal(`Expected at least one line containing "Image successfully pushed"`)
	}
}

func (s *DockerRegistrySuite) TestPushInterrupt(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if err := pushCmd.Start(); err != nil {
		c.Fatalf("Failed to start pushing to private registry: %v", err)
	}

	// Interrupt push (yes, we have no idea at what point it will get killed).
	time.Sleep(200 * time.Millisecond)
	if err := pushCmd.Process.Kill(); err != nil {
		c.Fatalf("Failed to kill push process: %v", err)
	}
	if out, _, err := dockerCmdWithError(c, "push", repoName); err == nil {
		if !strings.Contains(out, "already in progress") {
			c.Fatalf("Push should be continued on daemon side, but seems ok: %v, %s", err, out)
		}
	}
	// now wait until all this pushes will complete
	// if it failed with timeout - there would be some error,
	// so no logic about it here
	for exec.Command(dockerBinary, "push", repoName).Run() != nil {
	}
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
	if out, _, err := dockerCmdWithError(c, "push", repoName); err != nil {
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

func (s *DockerTrustSuite) TestTrustedPushWithoutServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithServer(pushCmd, "example/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Missing error while running trusted push w/ no server")
	}

	if !strings.Contains(string(out), "Error establishing connection to notary repository") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithoutServerAndUntrusted(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", "--untrusted", repoName)
	s.trustedCmdWithServer(pushCmd, "example/")
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("trusted push with no server and --untrusted failed: %s\n%s", err, out)
	}

	if strings.Contains(string(out), "Error establishing connection to notary repository") {
		c.Fatalf("Missing expected output on trusted push with --untrusted:\n%s", out)
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

func (s *DockerTrustSuite) TestTrustedPushWithShortRootPassphrase(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "rootPwd", "", "")
	out, _, err := runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short root passphrase")
	}

	if !strings.Contains(string(out), "tuf: insufficient signatures for Cryptoservice") {
		c.Fatalf("Missing expected output on trusted push with short root passphrase:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedPushWithIncorrectRootPassphrase(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	// Push with default passphrase
	pushCmd := exec.Command(dockerBinary, "push", "--untrusted", repoName)
	s.trustedCmd(pushCmd)
	out, _, _ := runCommandWithOutput(pushCmd)
	fmt.Println("OUTPUT: ", out)

	// Push with incorrect passphrase
	pushCmd = exec.Command(dockerBinary, "push", "--untrusted", repoName)
	s.trustedCmd(pushCmd)
	// s.trustedCmdWithPassphrases(pushCmd, "87654321", "", "")
	out, _, _ = runCommandWithOutput(pushCmd)
	fmt.Println("OUTPUT2:", out)
	//c.Fail()
}

func (s *DockerTrustSuite) TestTrustedPushWithShortPassphraseForNonRoot(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmdWithPassphrases(pushCmd, "12345678", "short", "short")
	out, _, err := runCommandWithOutput(pushCmd)
	if err == nil {
		c.Fatalf("Error missing from trusted push with short targets passphrase")
	}

	if !strings.Contains(string(out), "tuf: insufficient signatures for Cryptoservice") {
		c.Fatalf("Missing expected output on trusted push with short targets/snapsnot passphrase:\n%s", out)
	}
}
