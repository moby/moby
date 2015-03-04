package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

var (
	repoName    = fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	digestRegex = regexp.MustCompile("Digest: ([^\n]+)")
)

func setupImage() (string, error) {
	containerName := "busyboxbydigest"

	c := exec.Command(dockerBinary, "run", "-d", "-e", "digest=1", "--name", containerName, "busybox")
	if _, err := runCommand(c); err != nil {
		return "", err
	}

	// tag the image to upload it to the private registry
	c = exec.Command(dockerBinary, "commit", containerName, repoName)
	if out, _, err := runCommandWithOutput(c); err != nil {
		return "", fmt.Errorf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoName)

	// delete the container as we don't need it any more
	if err := deleteContainer(containerName); err != nil {
		return "", err
	}

	// push the image
	c = exec.Command(dockerBinary, "push", repoName)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		return "", fmt.Errorf("pushing the image to the private registry has failed: %s, %v", out, err)
	}

	// delete busybox and our local repo that we previously tagged
	//if err := deleteImages(repoName, "busybox"); err != nil {
	c = exec.Command(dockerBinary, "rmi", repoName, "busybox")
	if out, _, err := runCommandWithOutput(c); err != nil {
		return "", fmt.Errorf("error deleting images prior to real test: %s, %v", out, err)
	}

	// the push output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	if len(matches) != 2 {
		return "", fmt.Errorf("unable to parse digest from push output: %s", out)
	}
	pushDigest := matches[1]

	return pushDigest, nil
}

func TestPullByDigest(t *testing.T) {
	defer setupRegistry(t)()

	pushDigest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	// pull from the registry using the <name>@<digest> reference
	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)
	c := exec.Command(dockerBinary, "pull", imageReference)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}
	defer deleteImages(imageReference)

	// the pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	if len(matches) != 2 {
		t.Fatalf("unable to parse digest from pull output: %s", out)
	}
	pullDigest := matches[1]

	// make sure the pushed and pull digests match
	if pushDigest != pullDigest {
		t.Fatalf("push digest %q didn't match pull digest %q", pushDigest, pullDigest)
	}

	logDone("by_digest - pull by digest")
}

func TestCreateByDigest(t *testing.T) {
	defer setupRegistry(t)()

	pushDigest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)

	containerName := "createByDigest"
	c := exec.Command(dockerBinary, "create", "--name", containerName, imageReference)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error creating by digest: %s, %v", out, err)
	}
	defer deleteContainer(containerName)

	res, err := inspectField(containerName, "Config.Image")
	if err != nil {
		t.Fatalf("failed to get Config.Image: %s, %v", out, err)
	}
	if res != imageReference {
		t.Fatalf("unexpected Config.Image: %s (expected %s)", res, imageReference)
	}

	logDone("by_digest - create by digest")
}

func TestRunByDigest(t *testing.T) {
	defer setupRegistry(t)()

	pushDigest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)

	containerName := "runByDigest"
	c := exec.Command(dockerBinary, "run", "--name", containerName, imageReference, "sh", "-c", "echo found=$digest")
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error run by digest: %s, %v", out, err)
	}
	defer deleteContainer(containerName)

	foundRegex := regexp.MustCompile("found=([^\n]+)")
	matches := foundRegex.FindStringSubmatch(out)
	if len(matches) != 2 {
		t.Fatalf("error locating expected 'found=1' output: %s", out)
	}
	if matches[1] != "1" {
		t.Fatalf("Expected %q, got %q", "1", matches[1])
	}

	res, err := inspectField(containerName, "Config.Image")
	if err != nil {
		t.Fatalf("failed to get Config.Image: %s, %v", out, err)
	}
	if res != imageReference {
		t.Fatalf("unexpected Config.Image: %s (expected %s)", res, imageReference)
	}

	logDone("by_digest - run by digest")
}

func TestRemoveImageByDigest(t *testing.T) {
	defer setupRegistry(t)()

	digest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	c := exec.Command(dockerBinary, "pull", imageReference)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// make sure inspect runs ok
	if _, err := inspectField(imageReference, "Id"); err != nil {
		t.Fatalf("failed to inspect image: %v", err)
	}

	// do the delete
	if err := deleteImages(imageReference); err != nil {
		t.Fatalf("unexpected error deleting image: %v", err)
	}

	// try to inspect again - it should error this time
	if _, err := inspectField(imageReference, "Id"); err == nil {
		t.Fatalf("unexpected nil err trying to inspect what should be a non-existent image")
	} else if !strings.Contains(err.Error(), "No such image") {
		t.Fatalf("expected 'No such image' output, got %v", err)
	}

	logDone("by_digest - remove image by digest")
}

func TestBuildByDigest(t *testing.T) {
	defer setupRegistry(t)()

	digest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	c := exec.Command(dockerBinary, "pull", imageReference)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// get the image id
	imageID, err := inspectField(imageReference, "Id")
	if err != nil {
		t.Fatalf("error getting image id: %v", err)
	}

	// do the build
	name := "buildbydigest"
	defer deleteImages(name)
	_, err = buildImage(name, fmt.Sprintf(
		`FROM %s
     CMD ["/bin/echo", "Hello World"]`, imageReference),
		true)
	if err != nil {
		t.Fatal(err)
	}

	// get the build's image id
	res, err := inspectField(name, "Config.Image")
	if err != nil {
		t.Fatal(err)
	}
	// make sure they match
	if res != imageID {
		t.Fatalf("Image %s, expected %s", res, imageID)
	}

	logDone("by_digest - build by digest")
}

func TestTagByDigest(t *testing.T) {
	defer setupRegistry(t)()

	digest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	c := exec.Command(dockerBinary, "pull", imageReference)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// tag it
	tag := "tagbydigest"
	c = exec.Command(dockerBinary, "tag", imageReference, tag)
	if _, err := runCommand(c); err != nil {
		t.Fatalf("unexpected error tagging: %v", err)
	}

	expectedID, err := inspectField(imageReference, "Id")
	if err != nil {
		t.Fatalf("error getting original image id: %v", err)
	}

	tagID, err := inspectField(tag, "Id")
	if err != nil {
		t.Fatalf("error getting tagged image id: %v", err)
	}

	if tagID != expectedID {
		t.Fatalf("expected image id %q, got %q", expectedID, tagID)
	}

	logDone("by_digest - tag by digest")
}
