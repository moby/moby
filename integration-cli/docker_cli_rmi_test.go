package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/pkg/stringid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLIRmiSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIRmiSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIRmiSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIRmiSuite) TestRmiWithContainerFails(c *testing.T) {
	errSubstr := "is using it"

	// create a container
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	cleanedContainerID := strings.TrimSpace(out)

	// try to delete the image
	out, _, err := dockerCmdWithError("rmi", "busybox")
	// Container is using image, should not be able to rmi
	assert.ErrorContains(c, err, "")
	// Container is using image, error message should contain errSubstr
	assert.Assert(c, strings.Contains(out, errSubstr), "Container: %q", cleanedContainerID)
	// make sure it didn't delete the busybox name
	images, _ := dockerCmd(c, "images")
	// The name 'busybox' should not have been removed from images
	assert.Assert(c, strings.Contains(images, "busybox"))
}

func (s *DockerCLIRmiSuite) TestRmiTag(c *testing.T) {
	imagesBefore, _ := dockerCmd(c, "images", "-a")
	dockerCmd(c, "tag", "busybox", "utest:tag1")
	dockerCmd(c, "tag", "busybox", "utest/docker:tag2")
	dockerCmd(c, "tag", "busybox", "utest:5000/docker:tag3")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n")+3, fmt.Sprintf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter))
	}
	dockerCmd(c, "rmi", "utest/docker:tag2")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n")+2, fmt.Sprintf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter))
	}
	dockerCmd(c, "rmi", "utest:5000/docker:tag3")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n")+1, fmt.Sprintf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter))
	}
	dockerCmd(c, "rmi", "utest:tag1")
	{
		imagesAfter, _ := dockerCmd(c, "images", "-a")
		assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n"), fmt.Sprintf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter))
	}
}

func (s *DockerCLIRmiSuite) TestRmiImgIDMultipleTag(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir '/busybox-one'").Combined()
	containerID := strings.TrimSpace(out)

	// Wait for it to exit as cannot commit a running container on Windows, and
	// it will take a few seconds to exit
	if testEnv.OSType == "windows" {
		cli.WaitExited(c, containerID, 60*time.Second)
	}

	cli.DockerCmd(c, "commit", containerID, "busybox-one")

	imagesBefore := cli.DockerCmd(c, "images", "-a").Combined()
	cli.DockerCmd(c, "tag", "busybox-one", "busybox-one:tag1")
	cli.DockerCmd(c, "tag", "busybox-one", "busybox-one:tag2")

	imagesAfter := cli.DockerCmd(c, "images", "-a").Combined()
	// tag busybox to create 2 more images with same imageID
	assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n")+2, fmt.Sprintf("docker images shows: %q\n", imagesAfter))

	imgID := inspectField(c, "busybox-one:tag1", "Id")

	// run a container with the image
	out = runSleepingContainerInImage(c, "busybox-one")
	containerID = strings.TrimSpace(out)

	// first checkout without force it fails
	// rmi tagged in multiple repos should have failed without force
	cli.Docker(cli.Args("rmi", imgID)).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      fmt.Sprintf("conflict: unable to delete %s (cannot be forced) - image is being used by running container %s", stringid.TruncateID(imgID), stringid.TruncateID(containerID)),
	})

	cli.DockerCmd(c, "stop", containerID)
	cli.DockerCmd(c, "rmi", "-f", imgID)

	imagesAfter = cli.DockerCmd(c, "images", "-a").Combined()
	// rmi -f failed, image still exists
	assert.Assert(c, !strings.Contains(imagesAfter, imgID[:12]), "ImageID:%q; ImagesAfter: %q", imgID, imagesAfter)
}

func (s *DockerCLIRmiSuite) TestRmiImgIDForce(c *testing.T) {
	out := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir '/busybox-test'").Combined()
	containerID := strings.TrimSpace(out)

	// Wait for it to exit as cannot commit a running container on Windows, and
	// it will take a few seconds to exit
	if testEnv.OSType == "windows" {
		cli.WaitExited(c, containerID, 60*time.Second)
	}

	cli.DockerCmd(c, "commit", containerID, "busybox-test")

	imagesBefore := cli.DockerCmd(c, "images", "-a").Combined()
	cli.DockerCmd(c, "tag", "busybox-test", "utest:tag1")
	cli.DockerCmd(c, "tag", "busybox-test", "utest:tag2")
	cli.DockerCmd(c, "tag", "busybox-test", "utest/docker:tag3")
	cli.DockerCmd(c, "tag", "busybox-test", "utest:5000/docker:tag4")
	{
		imagesAfter := cli.DockerCmd(c, "images", "-a").Combined()
		assert.Equal(c, strings.Count(imagesAfter, "\n"), strings.Count(imagesBefore, "\n")+4, fmt.Sprintf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter))
	}
	imgID := inspectField(c, "busybox-test", "Id")

	// first checkout without force it fails
	cli.Docker(cli.Args("rmi", imgID)).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "(must be forced) - image is referenced in multiple repositories",
	})

	cli.DockerCmd(c, "rmi", "-f", imgID)
	{
		imagesAfter := cli.DockerCmd(c, "images", "-a").Combined()
		// rmi failed, image still exists
		assert.Assert(c, !strings.Contains(imagesAfter, imgID[:12]))
	}
}

// See https://github.com/docker/docker/issues/14116
func (s *DockerCLIRmiSuite) TestRmiImageIDForceWithRunningContainersAndMultipleTags(c *testing.T) {
	dockerfile := "FROM busybox\nRUN echo test 14116\n"
	buildImageSuccessfully(c, "test-14116", build.WithDockerfile(dockerfile))
	imgID := getIDByName(c, "test-14116")

	newTag := "newtag"
	dockerCmd(c, "tag", imgID, newTag)
	runSleepingContainerInImage(c, imgID)

	out, _, err := dockerCmdWithError("rmi", "-f", imgID)
	// rmi -f should not delete image with running containers
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "(cannot be forced) - image is being used by running container"))
}

func (s *DockerCLIRmiSuite) TestRmiTagWithExistingContainers(c *testing.T) {
	container := "test-delete-tag"
	newtag := "busybox:newtag"
	bb := "busybox:latest"
	dockerCmd(c, "tag", bb, newtag)

	dockerCmd(c, "run", "--name", container, bb, "/bin/true")

	out, _ := dockerCmd(c, "rmi", newtag)
	assert.Equal(c, strings.Count(out, "Untagged: "), 1)
}

func (s *DockerCLIRmiSuite) TestRmiForceWithExistingContainers(c *testing.T) {
	image := "busybox-clone"

	icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "build", "--no-cache", "-t", image, "-"},
		Stdin: strings.NewReader(`FROM busybox
MAINTAINER foo`),
	}).Assert(c, icmd.Success)

	dockerCmd(c, "run", "--name", "test-force-rmi", image, "/bin/true")

	dockerCmd(c, "rmi", "-f", image)
}

func (s *DockerCLIRmiSuite) TestRmiWithMultipleRepositories(c *testing.T) {
	newRepo := "127.0.0.1:5000/busybox"
	oldRepo := "busybox"
	newTag := "busybox:test"
	dockerCmd(c, "tag", oldRepo, newRepo)

	dockerCmd(c, "run", "--name", "test", oldRepo, "touch", "/abcd")

	dockerCmd(c, "commit", "test", newTag)

	out, _ := dockerCmd(c, "rmi", newTag)
	assert.Assert(c, strings.Contains(out, "Untagged: "+newTag))
}

func (s *DockerCLIRmiSuite) TestRmiForceWithMultipleRepositories(c *testing.T) {
	imageName := "rmiimage"
	tag1 := imageName + ":tag1"
	tag2 := imageName + ":tag2"

	buildImageSuccessfully(c, tag1, build.WithDockerfile(`FROM busybox
		MAINTAINER "docker"`))
	dockerCmd(c, "tag", tag1, tag2)

	out, _ := dockerCmd(c, "rmi", "-f", tag2)
	assert.Assert(c, strings.Contains(out, "Untagged: "+tag2))
	assert.Assert(c, !strings.Contains(out, "Untagged: "+tag1))
	// Check built image still exists
	images, _ := dockerCmd(c, "images", "-a")
	assert.Assert(c, strings.Contains(images, imageName), "Built image missing %q; Images: %q", imageName, images)
}

func (s *DockerCLIRmiSuite) TestRmiBlank(c *testing.T) {
	out, _, err := dockerCmdWithError("rmi", " ")
	// Should have failed to delete ' ' image
	assert.ErrorContains(c, err, "")
	// Wrong error message generated
	assert.Assert(c, !strings.Contains(out, "no such id"), "out: %s", out)
	// Expected error message not generated
	assert.Assert(c, strings.Contains(out, "image name cannot be blank"), "out: %s", out)
}

func (s *DockerCLIRmiSuite) TestRmiContainerImageNotFound(c *testing.T) {
	// Build 2 images for testing.
	imageNames := []string{"test1", "test2"}
	imageIds := make([]string, 2)
	for i, name := range imageNames {
		dockerfile := fmt.Sprintf("FROM busybox\nMAINTAINER %s\nRUN echo %s\n", name, name)
		buildImageSuccessfully(c, name, build.WithoutCache, build.WithDockerfile(dockerfile))
		id := getIDByName(c, name)
		imageIds[i] = id
	}

	// Create a long-running container.
	runSleepingContainerInImage(c, imageNames[0])

	// Create a stopped container, and then force remove its image.
	dockerCmd(c, "run", imageNames[1], "true")
	dockerCmd(c, "rmi", "-f", imageIds[1])

	// Try to remove the image of the running container and see if it fails as expected.
	out, _, err := dockerCmdWithError("rmi", "-f", imageIds[0])
	// The image of the running container should not be removed.
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "image is being used by running container"), "out: %s", out)
}

// #13422
func (s *DockerCLIRmiSuite) TestRmiUntagHistoryLayer(c *testing.T) {
	image := "tmp1"
	// Build an image for testing.
	dockerfile := `FROM busybox
MAINTAINER foo
RUN echo 0 #layer0
RUN echo 1 #layer1
RUN echo 2 #layer2
`
	buildImageSuccessfully(c, image, build.WithoutCache, build.WithDockerfile(dockerfile))
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
	// Expected 1 untagged entry
	assert.Equal(c, strings.Count(out, "Untagged: "), 1, fmt.Sprintf("out: %s", out))

	// Now let's add the tag again and create a container based on it.
	dockerCmd(c, "tag", idToTag, newTag)
	out, _ = dockerCmd(c, "run", "-d", newTag, "true")
	cid := strings.TrimSpace(out)

	// At this point we have 2 containers, one based on layer2 and another based on layer0.
	// Try to untag "tmp2" without the -f flag.
	out, _, err := dockerCmdWithError("rmi", newTag)
	// should not be untagged without the -f flag
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, cid[:12]))
	assert.Assert(c, strings.Contains(out, "(must force)"))
	// Add the -f flag and test again.
	out, _ = dockerCmd(c, "rmi", "-f", newTag)
	// should be allowed to untag with the -f flag
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("Untagged: %s:latest", newTag)))
}

func (*DockerCLIRmiSuite) TestRmiParentImageFail(c *testing.T) {
	buildImageSuccessfully(c, "test", build.WithDockerfile(`
	FROM busybox
	RUN echo hello`))

	id := inspectField(c, "busybox", "ID")
	out, _, err := dockerCmdWithError("rmi", id)
	assert.ErrorContains(c, err, "")
	if !strings.Contains(out, "image has dependent child images") {
		c.Fatalf("rmi should have failed because it's a parent image, got %s", out)
	}
}

func (s *DockerCLIRmiSuite) TestRmiWithParentInUse(c *testing.T) {
	out, _ := dockerCmd(c, "create", "busybox")
	cID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "commit", cID)
	imageID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "create", imageID)
	cID = strings.TrimSpace(out)

	out, _ = dockerCmd(c, "commit", cID)
	imageID = strings.TrimSpace(out)

	dockerCmd(c, "rmi", imageID)
}

// #18873
func (s *DockerCLIRmiSuite) TestRmiByIDHardConflict(c *testing.T) {
	dockerCmd(c, "create", "busybox")

	imgID := inspectField(c, "busybox:latest", "Id")

	_, _, err := dockerCmdWithError("rmi", imgID[:12])
	assert.ErrorContains(c, err, "")

	// check that tag was not removed
	imgID2 := inspectField(c, "busybox:latest", "Id")
	assert.Equal(c, imgID, imgID2)
}
