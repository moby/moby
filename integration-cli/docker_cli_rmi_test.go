package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestImageRemoveWithContainerFails(t *testing.T) {
	errSubstr := "is using it"

	// create a container
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	errorOut(err, t, fmt.Sprintf("failed to create a container: %v %v", out, err))

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
	images, _, _ := cmd(t, "images")
	if !strings.Contains(images, "busybox") {
		t.Fatalf("The name 'busybox' should not have been removed from images: %q", images)
	}

	deleteContainer(cleanedContainerID)

	logDone("rmi- container using image while rmi, should not remove image name")
}

func TestImageTagRemove(t *testing.T) {
	imagesBefore, _, _ := cmd(t, "images", "-a")
	cmd(t, "tag", "busybox", "utest:tag1")
	cmd(t, "tag", "busybox", "utest/docker:tag2")
	cmd(t, "tag", "busybox", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+3 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}
	}
	cmd(t, "rmi", "utest/docker:tag2")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+2 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	cmd(t, "rmi", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+1 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	cmd(t, "rmi", "utest:tag1")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+0 {
			t.Fatalf("before: %q\n\nafter: %q\n", imagesBefore, imagesAfter)
		}

	}
	logDone("tag,rmi- tagging the same images multiple times then removing tags")
}
