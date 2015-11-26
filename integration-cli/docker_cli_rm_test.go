package main

import (
	"os"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestRmContainerWithRemovedVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon)

	dockerCmd(c, "run", "--name", "losemyvolumes", "-v", "/tmp/testing:/test", "busybox", "true")

	err := os.Remove("/tmp/testing")
	c.Assert(err, check.IsNil)

	dockerCmd(c, "rm", "-v", "losemyvolumes")
}

func (s *DockerSuite) TestRmContainerWithVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "foo", "-v", "/srv", "busybox", "true")

	dockerCmd(c, "rm", "-v", "foo")
}

func (s *DockerSuite) TestRmRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	createRunningContainer(c, "foo")

	_, _, err := dockerCmdWithError("rm", "foo")
	c.Assert(err, checker.NotNil, check.Commentf("Expected error, can't rm a running container"))
}

func (s *DockerSuite) TestRmForceRemoveRunningContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	createRunningContainer(c, "foo")

	// Stop then remove with -s
	dockerCmd(c, "rm", "-f", "foo")
}

func (s *DockerSuite) TestRmContainerOrphaning(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerfile1 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]`
	img := "test-container-orphaning"
	dockerfile2 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]
	MAINTAINER Integration Tests`

	// build first dockerfile
	img1, err := buildImage(img, dockerfile1, true)
	c.Assert(err, check.IsNil, check.Commentf("Could not build image %s", img))
	// run container on first image
	dockerCmd(c, "run", img)
	// rebuild dockerfile with a small addition at the end
	_, err = buildImage(img, dockerfile2, true)
	c.Assert(err, check.IsNil, check.Commentf("Could not rebuild image %s", img))
	// try to remove the image, should not error out.
	out, _, err := dockerCmdWithError("rmi", img)
	c.Assert(err, check.IsNil, check.Commentf("Expected to removing the image, but failed: %s", out))

	// check if we deleted the first image
	out, _ = dockerCmd(c, "images", "-q", "--no-trunc")
	c.Assert(out, checker.Contains, img1, check.Commentf("Orphaned container (could not find %q in docker images): %s", img1, out))

}

func (s *DockerSuite) TestRmInvalidContainer(c *check.C) {
	if out, _, err := dockerCmdWithError("rm", "unknown"); err == nil {
		c.Fatal("Expected error on rm unknown container, got none")
	} else if !strings.Contains(out, "failed to remove containers") {
		c.Fatalf("Expected output to contain 'failed to remove containers', got %q", out)
	}
}

func createRunningContainer(c *check.C, name string) {
	dockerCmd(c, "run", "-dt", "--name", name, "busybox", "top")
}
