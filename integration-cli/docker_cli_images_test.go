package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/common"
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

func TestImagesFilterLabel(t *testing.T) {
	imageName1 := "images_filter_test1"
	imageName2 := "images_filter_test2"
	imageName3 := "images_filter_test3"
	defer deleteAllContainers()
	defer deleteImages(imageName1)
	defer deleteImages(imageName2)
	defer deleteImages(imageName3)
	image1ID, err := buildImage(imageName1,
		`FROM scratch
		 LABEL match me`, true)
	if err != nil {
		t.Fatal(err)
	}

	image2ID, err := buildImage(imageName2,
		`FROM scratch
		 LABEL match="me too"`, true)
	if err != nil {
		t.Fatal(err)
	}

	image3ID, err := buildImage(imageName3,
		`FROM scratch
		 LABEL nomatch me`, true)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "images", "--no-trunc", "-q", "-f", "label=match")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	out = strings.TrimSpace(out)

	if (!strings.Contains(out, image1ID) && !strings.Contains(out, image2ID)) || strings.Contains(out, image3ID) {
		t.Fatalf("Expected ids %s,%s got %s", image1ID, image2ID, out)
	}

	cmd = exec.Command(dockerBinary, "images", "--no-trunc", "-q", "-f", "label=match=me too")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	out = strings.TrimSpace(out)

	if out != image2ID {
		t.Fatalf("Expected %s got %s", image2ID, out)
	}

	logDone("images - filter label")
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
		cmd := exec.Command(dockerBinary, "images", "-q", "-f", filter)
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

func TestImagesEnsureDanglingImageOnlyListedOnce(t *testing.T) {
	defer deleteAllContainers()

	// create container 1
	c := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error running busybox: %s, %v", out, err)
	}
	containerId1 := strings.TrimSpace(out)

	// tag as foobox
	c = exec.Command(dockerBinary, "commit", containerId1, "foobox")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error tagging foobox: %s", err)
	}
	imageId := common.TruncateID(strings.TrimSpace(out))
	defer deleteImages(imageId)

	// overwrite the tag, making the previous image dangling
	c = exec.Command(dockerBinary, "tag", "-f", "busybox", "foobox")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error tagging foobox: %s", err)
	}
	defer deleteImages("foobox")

	c = exec.Command(dockerBinary, "images", "-q", "-f", "dangling=true")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("listing images failed with errors: %s, %v", out, err)
	}

	if e, a := 1, strings.Count(out, imageId); e != a {
		t.Fatalf("expected 1 dangling image, got %d: %s", a, out)
	}

	logDone("images - dangling image only listed once")
}
