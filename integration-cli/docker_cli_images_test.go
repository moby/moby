package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"sort"
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

func TestImagesErrorWithInvalidFilterNameTest(t *testing.T) {
	imagesCmd := exec.Command(dockerBinary, "images", "-f", "FOO=123")
	out, _, err := runCommandWithOutput(imagesCmd)
	if !strings.Contains(out, "Invalid filter") {
		t.Fatalf("error should occur when listing images with invalid filter name FOO, %s, %v", out, err)
	}

	logDone("images - invalid filter name check working")
}

func TestImagesFilterWhiteSpaceTrimmingAndLowerCasingWorking(t *testing.T) {
	imageName := "images_filter_test"
	defer deleteAllContainers()
	defer deleteImages(imageName)
	buildImage(imageName,
		`FROM scratch
		 RUN touch /test/foo
		 RUN touch /test/bar
		 RUN touch /test/baz`, true)

	filters := []string{
		"dangling=true",
		"Dangling=true",
		" dangling=true",
		"dangling=true ",
		"dangling = true",
	}

	imageListings := make([][]string, 5, 5)
	for idx, filter := range filters {
		cmd := exec.Command(dockerBinary, "images", "-f", filter)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			t.Fatal(err)
		}
		listing := strings.Split(out, "\n")
		sort.Strings(listing)
		imageListings[idx] = listing
	}

	for idx, listing := range imageListings {
		if idx < 4 && !reflect.DeepEqual(listing, imageListings[idx+1]) {
			for idx, errListing := range imageListings {
				fmt.Printf("out %d", idx)
				for _, image := range errListing {
					fmt.Print(image)
				}
				fmt.Print("")
			}
			t.Fatalf("All output must be the same")
		}
	}

	logDone("images - white space trimming and lower casing")
}
