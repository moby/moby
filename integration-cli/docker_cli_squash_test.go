package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

func squashImage(c *check.C, image, ancestor, tag string) (squashedImageID string) {
	args := []string{"squash", "--no-trunc"}

	if tag != "" {
		args = append(args, "--tag", tag)
	}

	args = append(args, image)

	if ancestor != "" {
		args = append(args, ancestor)
	}

	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	if err != nil || exitCode != 0 {
		c.Fatalf("failed to squash image: %s, %v", out, err)
	}

	return strings.TrimSpace(out)
}

func getImageHistory(c *check.C, image string) (layerIDs []string) {
	// Get image history.
	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "history", "-q", "--no-trunc", image))
	if err != nil || exitCode != 0 {
		c.Fatalf("failed to get image history: %s, %v", out, err)
	}

	return strings.Split(strings.TrimSpace(out), "\n")
}

// TestSquashNop tries to squash an image that can't actually be squashed any
// further either because it has no parent image layer or the specified
// ancestor is itself.
func (s *DockerSuite) TestSquashNop(c *check.C) {
	squashedBusyboxID := squashImage(c, "busybox", "", "")

	// Try squashing the squashed image again. This should produce the same
	// ID because it has no ancestor layers.
	squashedAgainID := squashImage(c, squashedBusyboxID, "", "")
	if squashedAgainID != squashedBusyboxID {
		c.Fatalf("expected squashing the image again to produce the same ID %q, but got %q", squashedBusyboxID, squashedAgainID)
	}

	// Try squashing busybox to busybox. This should also produce the same ID
	// because ther are no ancestor layers between an image and itself.
	busyboxLayerIDs := getImageHistory(c, "busybox")
	squashedSelfID := squashImage(c, "busybox", "busybox", "")
	if squashedSelfID != busyboxLayerIDs[0] {
		c.Fatalf("expected squashing busybox to itself to produce the same ID %q, but got %q", busyboxLayerIDs[0], squashedSelfID)
	}
}

// TestSquashAbsolute ensures that an image can be squashed completely to a new
// single-layered image and that the new image behaves like the original.
func (s *DockerSuite) TestSquashAbsolute(c *check.C) {
	squashedBusyboxID := squashImage(c, "busybox", "", "")

	// Ensure that there is only a single layer in the new image.
	squashedImageHistory := getImageHistory(c, squashedBusyboxID)
	if len(squashedImageHistory) != 1 {
		c.Fatalf("expected absolutely-squashed image to have a single layer, it has %d.", len(squashedImageHistory))
	}

	if squashedBusyboxID != squashedImageHistory[0] {
		c.Fatalf("expected the squashed image history to be only %q, but it is %v", squashedBusyboxID, squashedImageHistory)
	}

	// Do a test run of the new image.
	out, exitCode, err := runCommandWithOutput(exec.Command(dockerBinary, "run", squashedBusyboxID, "echo", "hello"))
	if err != nil || exitCode != 0 {
		c.Fatalf("failed to run squashed image: %s, %v", out, err)
	}

	expectedOutput := "hello\n"
	if out != expectedOutput {
		c.Fatalf("unexpected output running squashed image: got %q, expected %q", out, expectedOutput)
	}
}

func imageHistoriesEqual(history1, history2 []string) bool {
	if len(history1) != len(history2) {
		return false
	}

	for i, val := range history1 {
		if val != history2[i] {
			return false
		}
	}

	return true
}

func (s *DockerSuite) TestSquashRelative(c *check.C) {
	// Build a new image on top of the official busybox image.
	testBuildImage := "testbuildimage"

	if _, err := buildImage(
		testBuildImage,
		`FROM busybox
MAINTAINER Tester McTest <tester@example.com>
RUN echo hello foo > /root/foo.txt
RUN echo hello bar > /root/bar.txt
RUN echo hello baz > /root/baz.txt
RUN rm /root/bar.txt`,
		false,
	); err != nil {
		c.Fatalf("unable to build test image: %v", err)
	}

	busyboxLayers := getImageHistory(c, "busybox")
	testBuildImageLayers := getImageHistory(c, testBuildImage)

	numNewLayers := len(testBuildImageLayers) - len(busyboxLayers)
	if numNewLayers != 5 {
		c.Fatalf("expected the build to add 5 new layers but it added %d", numNewLayers)
	}

	// Squash the newly built image up to its base image, "busybox".
	testBuildImageSquashed := squashImage(c, testBuildImage, "busybox", "")
	squashedImageHistory := getImageHistory(c, testBuildImageSquashed)

	expectedHistory := append([]string{testBuildImageSquashed}, busyboxLayers...)
	if !imageHistoriesEqual(expectedHistory, squashedImageHistory) {
		c.Fatalf("expected the squashed image history to be %v, but it is %v", expectedHistory, squashedImageHistory)
	}

	// Now ensure that the squashed image behaves the same as the unsquashed
	// image.
	expectedLsRootOutput, statusCode, err := runCommandWithOutput(exec.Command(dockerBinary, "run", testBuildImage, "ls", "/root"))
	if err != nil || statusCode != 0 {
		c.Fatalf("failed to 'ls /root' in %q: %s, %v", testBuildImage, expectedLsRootOutput, err)
	}

	squashedLSRootOutput, statusCode, err := runCommandWithOutput(exec.Command(dockerBinary, "run", testBuildImageSquashed, "ls", "/root"))
	if err != nil || statusCode != 0 {
		c.Fatalf("failed to 'ls /root' in %q: %s, %v", testBuildImageSquashed, squashedLSRootOutput, err)
	}

	if squashedLSRootOutput != expectedLsRootOutput {
		c.Fatalf("expected 'ls /root' of squashed image to be %q, but it is %q", expectedLsRootOutput, squashedLSRootOutput)
	}
}

func (s *DockerSuite) TestSquashTag(c *check.C) {
	testTag := "squashed/busybox:test"
	squashedBusyboxID := squashImage(c, "busybox", "", testTag)

	// Ensure that tagging worked by comparing history.
	historyByID := getImageHistory(c, squashedBusyboxID)
	historyByTag := getImageHistory(c, testTag)

	if !imageHistoriesEqual(historyByID, historyByTag) {
		c.Fatalf("expected image history to be %v, but it is %v", historyByID, historyByTag)
	}
}
