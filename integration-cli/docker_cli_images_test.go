package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestImagesEnsureImageIsListed(t *testing.T) {
	imagesCmd := exec.Command(dockerBinary, "images")
	out, _, err := runCommandWithOutput(imagesCmd)
	if err != nil {
		t.Fatalf("listing images failed with errors: %s, %v", out, err)
	}

	if !strings.Contains(out, "busybox") {
		t.Fatal("images should've listed busybox")
	}

	logDone("images - busybox should be listed")
}

func TestImagesOrderedByCreationDate(t *testing.T) {
	defer deleteImages("order:test_a")
	defer deleteImages("order:test_c")
	defer deleteImages("order:test_b")
	id1, err := buildImage("order:test_a",
		`FROM scratch
		MAINTAINER dockerio1`, true)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	id2, err := buildImage("order:test_c",
		`FROM scratch
		MAINTAINER dockerio2`, true)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	id3, err := buildImage("order:test_b",
		`FROM scratch
		MAINTAINER dockerio3`, true)
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "images", "-q", "--no-trunc"))
	if err != nil {
		t.Fatalf("listing images failed with errors: %s, %v", out, err)
	}
	imgs := strings.Split(out, "\n")
	if imgs[0] != id3 {
		t.Fatalf("First image must be %s, got %s", id3, imgs[0])
	}
	if imgs[1] != id2 {
		t.Fatalf("Second image must be %s, got %s", id2, imgs[1])
	}
	if imgs[2] != id1 {
		t.Fatalf("Third image must be %s, got %s", id1, imgs[2])
	}

	logDone("images - ordering by creation date")
}
