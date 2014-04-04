package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestImagesEnsureImageIsListed(t *testing.T) {
	imagesCmd := exec.Command(dockerBinary, "images")
	out, _, err := runCommandWithOutput(imagesCmd)
	errorOut(err, t, fmt.Sprintf("listing images failed with errors: %v", err))

	if !strings.Contains(out, "busybox") {
		t.Fatal("images should've listed busybox")
	}

	logDone("images - busybox should be listed")
}

func TestCLIImageTagRemove(t *testing.T) {
	imagesBefore, _, _ := cmd(t, "images", "-a")
	cmd(t, "tag", "busybox", "utest:tag1")
	cmd(t, "tag", "busybox", "utest/docker:tag2")
	cmd(t, "tag", "busybox", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+3 {
			t.Fatalf("before: %#s\n\nafter: %#s\n", imagesBefore, imagesAfter)
		}
	}
	cmd(t, "rmi", "utest/docker:tag2")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+2 {
			t.Fatalf("before: %#s\n\nafter: %#s\n", imagesBefore, imagesAfter)
		}

	}
	cmd(t, "rmi", "utest:5000/docker:tag3")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+1 {
			t.Fatalf("before: %#s\n\nafter: %#s\n", imagesBefore, imagesAfter)
		}

	}
	cmd(t, "rmi", "utest:tag1")
	{
		imagesAfter, _, _ := cmd(t, "images", "-a")
		if nLines(imagesAfter) != nLines(imagesBefore)+0 {
			t.Fatalf("before: %#s\n\nafter: %#s\n", imagesBefore, imagesAfter)
		}

	}
	logDone("tag,rmi- tagging the same images multiple times then removing tags")
}
