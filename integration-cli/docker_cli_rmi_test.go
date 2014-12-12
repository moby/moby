package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRmiWithContainerFails(t *testing.T) {
	errSubstr := "is using it"

	// create a container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("failed to create a container: %s, %v", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	// try to delete the image
	runCmd = exec.Command(dockerBinary, "rmi", "busybox")
	out, _, err = runCommandWithOutput(runCmd)
	if err == nil {
		t.Fatalf("Container %q is using image, should not be able to rmi: %q", cleanedContainerID, out)
	}
	if !strings.Contains(out, errSubstr) {
		t.Fatalf("Container %q is using image, error message should contain %q: %v", cleanedContainerID, errSubstr, out)
	}

	// make sure it didn't delete the busybox name
	images, _, _ := dockerCmd(t, "images")
	if !strings.Contains(images, "busybox") {
		t.Fatalf("The name 'busybox' should not have been removed from images: %q", images)
	}

	deleteContainer(cleanedContainerID)

	logDone("rmi- container using image while rmi, should not remove image name")
}

func TestRmiTag(t *testing.T) {
	imagesBefore, _, _ := dockerCmd(t, "images", "-a")
	dockerCmd(t, "tag", "busybox", "utest:tag1")
	dockerCmd(t, "tag", "busybox", "utest/docker:tag2")
	dockerCmd(t, "tag", "busybox", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := dockerCmd(t, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+3 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}
	}
	dockerCmd(t, "rmi", "utest/docker:tag2")
	{
		imagesAfter, _, _ := dockerCmd(t, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+2 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	dockerCmd(t, "rmi", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := dockerCmd(t, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+1 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	dockerCmd(t, "rmi", "utest:tag1")
	{
		imagesAfter, _, _ := dockerCmd(t, "images", "-a")
		if strings.Count(imagesAfter, "\n") != strings.Count(imagesBefore, "\n")+0 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	logDone("rmi - tag,rmi- tagging the same images multiple times then removing tags")
}

func TestRmiTagWithExistingContainers(t *testing.T) {
	container := "test-delete-tag"
	newtag := "busybox:newtag"
	bb := "busybox:latest"
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "tag", bb, newtag)); err != nil {
		t.Fatalf("Could not tag busybox: %v: %s", err, out)
	}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", container, bb, "/bin/true")); err != nil {
		t.Fatalf("Could not run busybox: %v: %s", err, out)
	}
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rmi", newtag))
	if err != nil {
		t.Fatalf("Could not remove tag %s: %v: %s", newtag, err, out)
	}
	if d := strings.Count(out, "Untagged: "); d != 1 {
		t.Fatalf("Expected 1 untagged entry got %d: %q", d, out)
	}

	deleteAllContainers()

	logDone("rmi - delete tag with existing containers")
}

func TestRmiForceWithExistingContainers(t *testing.T) {
	image := "busybox-clone"
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "build", "--no-cache", "-t", image, "/docker-busybox")); err != nil {
		t.Fatalf("Could not build %s: %s, %v", image, out, err)
	}

	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "test-force-rmi", image, "/bin/true")); err != nil {
		t.Fatalf("Could not run container: %s, %v", out, err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rmi", "-f", image))
	if err != nil {
		t.Fatalf("Could not remove image %s:  %s, %v", image, out, err)
	}

	deleteAllContainers()

	logDone("rmi - force delete with existing containers")
}
