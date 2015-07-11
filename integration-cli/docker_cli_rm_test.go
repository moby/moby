package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestRmContainerWithRemovedVolume(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "run", "--name", "losemyvolumes", "-v", "/tmp/testing:/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	if err := os.Remove("/tmp/testing"); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "losemyvolumes")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(out, err)
	}

}

func (s *DockerSuite) TestRmContainerWithVolume(c *check.C) {

	cmd := exec.Command(dockerBinary, "run", "--name", "foo", "-v", "/srv", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "foo")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestRmRunningContainer(c *check.C) {

	createRunningContainer(c, "foo")

	// Test cannot remove running container
	cmd := exec.Command(dockerBinary, "rm", "foo")
	if _, err := runCommand(cmd); err == nil {
		c.Fatalf("Expected error, can't rm a running container")
	}

}

func (s *DockerSuite) TestRmForceRemoveRunningContainer(c *check.C) {

	createRunningContainer(c, "foo")

	// Stop then remove with -s
	cmd := exec.Command(dockerBinary, "rm", "-f", "foo")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestRmContainerOrphaning(c *check.C) {

	dockerfile1 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]`
	img := "test-container-orphaning"
	dockerfile2 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]
	MAINTAINER Integration Tests`

	// build first dockerfile
	img1, err := buildImage(img, dockerfile1, true)
	if err != nil {
		c.Fatalf("Could not build image %s: %v", img, err)
	}
	// run container on first image
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", img)); err != nil {
		c.Fatalf("Could not run image %s: %v: %s", img, err, out)
	}
	// rebuild dockerfile with a small addition at the end
	if _, err := buildImage(img, dockerfile2, true); err != nil {
		c.Fatalf("Could not rebuild image %s: %v", img, err)
	}
	// try to remove the image, should error out.
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rmi", img)); err == nil {
		c.Fatalf("Expected to error out removing the image, but succeeded: %s", out)
	}
	// check if we deleted the first image
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "images", "-q", "--no-trunc"))
	if err != nil {
		c.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(out, img1) {
		c.Fatalf("Orphaned container (could not find %q in docker images): %s", img1, out)
	}

}

func (s *DockerSuite) TestRmInvalidContainer(c *check.C) {
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rm", "unknown")); err == nil {
		c.Fatal("Expected error on rm unknown container, got none")
	} else if !strings.Contains(out, "failed to remove containers") {
		c.Fatalf("Expected output to contain 'failed to remove containers', got %q", out)
	}

}

func createRunningContainer(c *check.C, name string) {
	cmd := exec.Command(dockerBinary, "run", "-dt", "--name", name, "busybox", "top")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}
