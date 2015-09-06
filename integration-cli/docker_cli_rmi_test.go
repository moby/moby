package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestRmiWithContainerFails(c *check.C) {
	testRequires(c, DaemonIsLinux)
	errSubstr := "is using it"

	// create a container
	out, _, err := dockerCmdWithError("run", "-d", "busybox", "true")
	if err != nil {
		c.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	// try to delete the image
	out, _, err = dockerCmdWithError("rmi", "busybox")
	if err == nil {
		c.Fatalf("Container %q is using image, should not be able to rmi: %q", cleanedContainerID, out)
	}
	if !strings.Contains(out, errSubstr) {
		c.Fatalf("Container %q is using image, error message should contain %q: %v", cleanedContainerID, errSubstr, out)
	}

	// make sure it didn't delete the busybox name
	images, _ := dockerCmd(c, "images")
	if !strings.Contains(images, "busybox") {
		c.Fatalf("The name 'busybox' should not have been removed from images: %q", images)
	}
}

func (s *DockerSuite) TestRmiTag(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imagesBefore, _ := dockerCmd(c, "images", "-a")
	dockerCmd(c, "tag", "busybox", "utest:tag1")
	dockerCmd(c, "tag", "busybox", "utest/docker:tag2")
	dockerCmd(c, "tag", "busybox", "utest:5000/docker:tag3")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+3 {
			c.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}
	}
	dockerCmd(c, "rmi", "utest/docker:tag2")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+2 {
			c.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	dockerCmd(c, "rmi", "utest:5000/docker:tag3")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+1 {
			c.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	dockerCmd(c, "rmi", "utest:tag1")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+0 {
			c.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
}

func (s *DockerSuite) TestRmiImgIDMultipleTag(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-d", "busybox", "/bin/sh", "-c", "mkdir '/busybox-one'")
	if err != nil {
		c.Fatalf("failed to create a container:%s, %v", out, err)
	}

	containerID := strings.TrimSpace(out)
	out, _, err = dockerCmdWithError("commit", containerID, "busybox-one")
	if err != nil {
		c.Fatalf("failed to commit a new busybox-one:%s, %v", out, err)
	}

	imagesBefore, _ := dockerCmd(c, "images", "-a")
	dockerCmd(c, "tag", "busybox-one", "busybox-one:tag1")
	dockerCmd(c, "tag", "busybox-one", "busybox-one:tag2")

	imagesAfter, _ := dockerCmd(c, "images", "-a")
	if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+2 {
		c.Fatalf("tag busybox to create 2 more images with same imageID; docker images shows: %q\n", imagesAfter)
	}

	imgID, err := inspectField("busybox-one:tag1", "Id")
	c.Assert(err, check.IsNil)

	// run a container with the image
	out, _, err = dockerCmdWithError("run", "-d", "busybox-one", "top")
	if err != nil {
		c.Fatalf("failed to create a container:%s, %v", out, err)
	}

	containerID = strings.TrimSpace(out)

	// first checkout without force it fails
	out, _, err = dockerCmdWithError("rmi", imgID)
	expected := fmt.Sprintf("conflict: unable to delete %s (cannot be forced) - image is being used by running container %s", imgID[:12], containerID[:12])
	if err == nil || !strings.Contains(out, expected) {
		c.Fatalf("rmi tagged in multiple repos should have failed without force: %s, %v, expected: %s", out, err, expected)
	}

	dockerCmd(c, "stop", containerID)
	dockerCmd(c, "rmi", "-f", imgID)

	imagesAfter, _ = dockerCmd(c, "images", "-a")
	if strings.Contains(imagesAfter, imgID[:12]) {
		c.Fatalf("rmi -f %s failed, image still exists: %q\n\n", imgID, imagesAfter)
	}
}

func (s *DockerSuite) TestRmiImgIDForce(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-d", "busybox", "/bin/sh", "-c", "mkdir '/busybox-test'")
	if err != nil {
		c.Fatalf("failed to create a container:%s, %v", out, err)
	}

	containerID := strings.TrimSpace(out)
	out, _, err = dockerCmdWithError("commit", containerID, "busybox-test")
	if err != nil {
		c.Fatalf("failed to commit a new busybox-test:%s, %v", out, err)
	}

	imagesBefore, _ := dockerCmd(c, "images", "-a")
	dockerCmd(c, "tag", "busybox-test", "utest:tag1")
	dockerCmd(c, "tag", "busybox-test", "utest:tag2")
	dockerCmd(c, "tag", "busybox-test", "utest/docker:tag3")
	dockerCmd(c, "tag", "busybox-test", "utest:5000/docker:tag4")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+4 {
			c.Fatalf("tag busybox to create 4 more images with same imageID; docker images shows: %q\n", imagesAfter)
		}
	}
	imgID, err := inspectField("busybox-test", "Id")
	c.Assert(err, check.IsNil)

	// first checkout without force it fails
	out, _, err = dockerCmdWithError("rmi", imgID)
	if err == nil || !strings.Contains(out, "(must be forced) - image is referenced in one or more repositories") {
		c.Fatalf("rmi tagged in multiple repos should have failed without force:%s, %v", out, err)
	}

	dockerCmd(c, "rmi", "-f", imgID)
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		if strings.Contains(imagesAfter, imgID[:12]) {
			c.Fatalf("rmi -f %s failed, image still exists: %q\n\n", imgID, imagesAfter)
		}
	}
}

// See https://github.com/docker/docker/issues/14116
func (s *DockerSuite) TestRmiImageIDForceWithRunningContainersAndMultipleTags(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerfile := "FROM busybox\nRUN echo test 14116\n"
	imgID, err := buildImage("test-14116", dockerfile, false)
	c.Assert(err, check.IsNil)

	newTag := "newtag"
	dockerCmd(c, "tag", imgID, newTag)
	dockerCmd(c, "run", "-d", imgID, "top")

	out, _, err := dockerCmdWithError("rmi", "-f", imgID)
	if err == nil || !strings.Contains(out, "(cannot be forced) - image is being used by running container") {
		c.Log(out)
		c.Fatalf("rmi -f should not delete image with running containers")
	}
}

func (s *DockerSuite) TestRmiTagWithExistingContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)
	container := "test-delete-tag"
	newtag := "busybox:newtag"
	bb := "busybox:latest"
	if out, _, err := dockerCmdWithError("tag", bb, newtag); err != nil {
		c.Fatalf("Could not tag busybox: %v: %s", err, out)
	}
	if out, _, err := dockerCmdWithError("run", "--name", container, bb, "/bin/true"); err != nil {
		c.Fatalf("Could not run busybox: %v: %s", err, out)
	}
	out, _, err := dockerCmdWithError("rmi", newtag)
	if err != nil {
		c.Fatalf("Could not remove tag %s: %v: %s", newtag, err, out)
	}
	if d := strings.Count(out, "Untagged: "); d != 1 {
		c.Fatalf("Expected 1 untagged entry got %d: %q", d, out)
	}
}

func (s *DockerSuite) TestRmiForceWithExistingContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)
	image := "busybox-clone"

	cmd := exec.Command(dockerBinary, "build", "--no-cache", "-t", image, "-")
	cmd.Stdin = strings.NewReader(`FROM busybox
MAINTAINER foo`)

	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatalf("Could not build %s: %s, %v", image, out, err)
	}

	if out, _, err := dockerCmdWithError("run", "--name", "test-force-rmi", image, "/bin/true"); err != nil {
		c.Fatalf("Could not run container: %s, %v", out, err)
	}

	if out, _, err := dockerCmdWithError("rmi", "-f", image); err != nil {
		c.Fatalf("Could not remove image %s:  %s, %v", image, out, err)
	}
}

func (s *DockerSuite) TestRmiWithMultipleRepositories(c *check.C) {
	testRequires(c, DaemonIsLinux)
	newRepo := "127.0.0.1:5000/busybox"
	oldRepo := "busybox"
	newTag := "busybox:test"
	out, _, err := dockerCmdWithError("tag", oldRepo, newRepo)
	if err != nil {
		c.Fatalf("Could not tag busybox: %v: %s", err, out)
	}

	out, _, err = dockerCmdWithError("run", "--name", "test", oldRepo, "touch", "/home/abcd")
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %s", err, out)
	}

	out, _, err = dockerCmdWithError("commit", "test", newTag)
	if err != nil {
		c.Fatalf("failed to commit container: %v, output: %s", err, out)
	}

	out, _, err = dockerCmdWithError("rmi", newTag)
	if err != nil {
		c.Fatalf("failed to remove image: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Untagged: "+newTag) {
		c.Fatalf("Could not remove image %s: %s, %v", newTag, out, err)
	}
}

func (s *DockerSuite) TestRmiBlank(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// try to delete a blank image name
	out, _, err := dockerCmdWithError("rmi", "")
	if err == nil {
		c.Fatal("Should have failed to delete '' image")
	}
	if strings.Contains(out, "no such id") {
		c.Fatalf("Wrong error message generated: %s", out)
	}
	if !strings.Contains(out, "image name cannot be blank") {
		c.Fatalf("Expected error message not generated: %s", out)
	}

	out, _, err = dockerCmdWithError("rmi", " ")
	if err == nil {
		c.Fatal("Should have failed to delete '' image")
	}
	if !strings.Contains(out, "no such id") {
		c.Fatalf("Expected error message not generated: %s", out)
	}
}

func (s *DockerSuite) TestRmiContainerImageNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Build 2 images for testing.
	imageNames := []string{"test1", "test2"}
	imageIds := make([]string, 2)
	for i, name := range imageNames {
		dockerfile := fmt.Sprintf("FROM busybox\nMAINTAINER %s\nRUN echo %s\n", name, name)
		id, err := buildImage(name, dockerfile, false)
		c.Assert(err, check.IsNil)
		imageIds[i] = id
	}

	// Create a long-running container.
	dockerCmd(c, "run", "-d", imageNames[0], "top")

	// Create a stopped container, and then force remove its image.
	dockerCmd(c, "run", imageNames[1], "true")
	dockerCmd(c, "rmi", "-f", imageIds[1])

	// Try to remove the image of the running container and see if it fails as expected.
	out, _, err := dockerCmdWithError("rmi", "-f", imageIds[0])
	if err == nil || !strings.Contains(out, "image is being used by running container") {
		c.Log(out)
		c.Fatal("The image of the running container should not be removed.")
	}
}

// #13422
func (s *DockerSuite) TestRmiUntagHistoryLayer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	image := "tmp1"
	// Build a image for testing.
	dockerfile := `FROM busybox
MAINTAINER foo
RUN echo 0 #layer0
RUN echo 1 #layer1
RUN echo 2 #layer2
`
	_, err := buildImage(image, dockerfile, false)
	c.Assert(err, check.IsNil)

	out, _ := dockerCmd(c, "history", "-q", image)
	ids := strings.Split(out, "\n")
	idToTag := ids[2]

	// Tag layer0 to "tmp2".
	newTag := "tmp2"
	dockerCmd(c, "tag", idToTag, newTag)
	// Create a container based on "tmp1".
	dockerCmd(c, "run", "-d", image, "true")

	// See if the "tmp2" can be untagged.
	out, _ = dockerCmd(c, "rmi", newTag)
	if d := strings.Count(out, "Untagged: "); d != 1 {
		c.Log(out)
		c.Fatalf("Expected 1 untagged entry got %d: %q", d, out)
	}

	// Now let's add the tag again and create a container based on it.
	dockerCmd(c, "tag", idToTag, newTag)
	out, _ = dockerCmd(c, "run", "-d", newTag, "true")
	cid := strings.TrimSpace(out)

	// At this point we have 2 containers, one based on layer2 and another based on layer0.
	// Try to untag "tmp2" without the -f flag.
	out, _, err = dockerCmdWithError("rmi", newTag)
	if err == nil || !strings.Contains(out, cid[:12]) || !strings.Contains(out, "(must force)") {
		c.Log(out)
		c.Fatalf("%q should not be untagged without the -f flag", newTag)
	}

	// Add the -f flag and test again.
	out, _ = dockerCmd(c, "rmi", "-f", newTag)
	if !strings.Contains(out, fmt.Sprintf("Untagged: %s:latest", newTag)) {
		c.Log(out)
		c.Fatalf("%q should be allowed to untag with the -f flag", newTag)
	}
}
