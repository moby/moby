package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
	"github.com/go-check/check"
)

// pulling an image from the central registry should work
func (s *DockerSuite) TestPushBusyboxImage(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	tagCmd := exec.Command(dockerBinary, "tag", "busybox", repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
}

// pushing an image without a prefix should throw an error
func (s *DockerSuite) TestPushUnprefixedRepo(c *check.C) {
	pushCmd := exec.Command(dockerBinary, "push", "busybox")
	if out, _, err := runCommandWithOutput(pushCmd); err == nil {
		c.Fatalf("pushing an unprefixed repo didn't result in a non-zero exit status: %s", out)
	}
}

func (s *DockerSuite) TestPushUntagged(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	expected := "Repository does not exist"
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: outuput %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerSuite) TestPushBadTag(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox:latest", privateRegistryURL)

	expected := "does not exist"
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err == nil {
		c.Fatalf("pushing the image to the private registry should have failed: outuput %q", out)
	} else if !strings.Contains(out, expected) {
		c.Fatalf("pushing the image failed with an unexpected message: expected %q, got %q", expected, out)
	}
}

func (s *DockerSuite) TestPushMultipleTags(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	repoTag1 := fmt.Sprintf("%v/dockercli/busybox:t1", privateRegistryURL)
	repoTag2 := fmt.Sprintf("%v/dockercli/busybox:t2", privateRegistryURL)
	// tag the image to upload it tot he private registry
	tagCmd1 := exec.Command(dockerBinary, "tag", "busybox", repoTag1)
	if out, _, err := runCommandWithOutput(tagCmd1); err != nil {
		c.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoTag1)
	tagCmd2 := exec.Command(dockerBinary, "tag", "busybox", repoTag2)
	if out, _, err := runCommandWithOutput(tagCmd2); err != nil {
		c.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoTag2)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
}

func (s *DockerSuite) TestPushInterrupt(c *check.C) {
	defer setupRegistry(c)()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it tot he private registry
	tagCmd := exec.Command(dockerBinary, "tag", "busybox", repoName)
	if out, _, err := runCommandWithOutput(tagCmd); err != nil {
		c.Fatalf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if err := pushCmd.Start(); err != nil {
		c.Fatalf("Failed to start pushing to private registry: %v", err)
	}

	// Interrupt push (yes, we have no idea at what point it will get killed).
	time.Sleep(200 * time.Millisecond)
	if err := pushCmd.Process.Kill(); err != nil {
		c.Fatalf("Failed to kill push process: %v", err)
	}
	// Try agin
	pushCmd = exec.Command(dockerBinary, "push", repoName)
	if out, err := pushCmd.CombinedOutput(); err == nil {
		str := string(out)
		if !strings.Contains(str, "already in progress") {
			c.Fatalf("Push should be continued on daemon side, but seems ok: %v, %s", err, out)
		}
	}
}

func (s *DockerSuite) TestPushEmptyLayer(c *check.C) {
	defer setupRegistry(c)()
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
	pushCmd := exec.Command(dockerBinary, "push", repoName)
	if out, _, err := runCommandWithOutput(pushCmd); err != nil {
		c.Fatalf("pushing the image to the private registry has failed: %s, %v", out, err)
	}
}
