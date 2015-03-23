package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/docker/docker/utils"
)

var (
	repoName    = fmt.Sprintf("%v/dockercli/busybox-by-dgst", privateRegistryURL)
	digestRegex = regexp.MustCompile("Digest: ([^\n]+)")
)

func setupImage() (string, error) {
	return setupImageWithTag("latest")
}

func setupImageWithTag(tag string) (string, error) {
	containerName := "busyboxbydigest"

	c := exec.Command(dockerBinary, "run", "-d", "-e", "digest=1", "--name", containerName, "busybox")
	if _, err := runCommand(c); err != nil {
		return "", err
	}

	// tag the image to upload it to the private registry
	repoAndTag := utils.ImageReference(repoName, tag)
	c = exec.Command(dockerBinary, "commit", containerName, repoAndTag)
	if out, _, err := runCommandWithOutput(c); err != nil {
		return "", fmt.Errorf("image tagging failed: %s, %v", out, err)
	}
	defer deleteImages(repoAndTag)

	// delete the container as we don't need it any more
	if err := deleteContainer(containerName); err != nil {
		return "", err
	}

	// push the image
	c = exec.Command(dockerBinary, "push", repoAndTag)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		return "", fmt.Errorf("pushing the image to the private registry has failed: %s, %v", out, err)
	}

	// delete our local repo that we previously tagged
	c = exec.Command(dockerBinary, "rmi", repoAndTag)
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

func TestPullByTagDisplaysDigest(t *testing.T) {
	defer setupRegistry(t)()

	pushDigest, err := setupImage()
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}

	// pull from the registry using the tag
	c := exec.Command(dockerBinary, "pull", repoName)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by tag: %s, %v", out, err)
	}
	defer deleteImages(repoName)

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

	logDone("by_digest - pull by tag displays digest")
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

func TestListImagesWithoutDigests(t *testing.T) {
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

	c = exec.Command(dockerBinary, "images")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	if strings.Contains(out, "DIGEST") {
		t.Fatalf("list output should not have contained DIGEST header: %s", out)
	}

	logDone("by_digest - list images - digest header not displayed by default")
}

func TestListImagesWithDigests(t *testing.T) {
	defer setupRegistry(t)()
	defer deleteImages(repoName+":tag1", repoName+":tag2")

	// setup image1
	digest1, err := setupImageWithTag("tag1")
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}
	imageReference1 := fmt.Sprintf("%s@%s", repoName, digest1)
	defer deleteImages(imageReference1)
	t.Logf("imageReference1 = %s", imageReference1)

	// pull image1 by digest
	c := exec.Command(dockerBinary, "pull", imageReference1)
	out, _, err := runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// list images
	c = exec.Command(dockerBinary, "images", "--digests")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	// make sure repo shown, tag=<none>, digest = $digest1
	re1 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest1 + `\s`)
	if !re1.MatchString(out) {
		t.Fatalf("expected %q: %s", re1.String(), out)
	}

	// setup image2
	digest2, err := setupImageWithTag("tag2")
	if err != nil {
		t.Fatalf("error setting up image: %v", err)
	}
	imageReference2 := fmt.Sprintf("%s@%s", repoName, digest2)
	defer deleteImages(imageReference2)
	t.Logf("imageReference2 = %s", imageReference2)

	// pull image1 by digest
	c = exec.Command(dockerBinary, "pull", imageReference1)
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// pull image2 by digest
	c = exec.Command(dockerBinary, "pull", imageReference2)
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling by digest: %s, %v", out, err)
	}

	// list images
	c = exec.Command(dockerBinary, "images", "--digests")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	// make sure repo shown, tag=<none>, digest = $digest1
	if !re1.MatchString(out) {
		t.Fatalf("expected %q: %s", re1.String(), out)
	}

	// make sure repo shown, tag=<none>, digest = $digest2
	re2 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest2 + `\s`)
	if !re2.MatchString(out) {
		t.Fatalf("expected %q: %s", re2.String(), out)
	}

	// pull tag1
	c = exec.Command(dockerBinary, "pull", repoName+":tag1")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling tag1: %s, %v", out, err)
	}

	// list images
	c = exec.Command(dockerBinary, "images", "--digests")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	// make sure image 1 has repo, tag, <none> AND repo, <none>, digest
	reWithTag1 := regexp.MustCompile(`\s*` + repoName + `\s*tag1\s*<none>\s`)
	reWithDigest1 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest1 + `\s`)
	if !reWithTag1.MatchString(out) {
		t.Fatalf("expected %q: %s", reWithTag1.String(), out)
	}
	if !reWithDigest1.MatchString(out) {
		t.Fatalf("expected %q: %s", reWithDigest1.String(), out)
	}
	// make sure image 2 has repo, <none>, digest
	if !re2.MatchString(out) {
		t.Fatalf("expected %q: %s", re2.String(), out)
	}

	// pull tag 2
	c = exec.Command(dockerBinary, "pull", repoName+":tag2")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error pulling tag2: %s, %v", out, err)
	}

	// list images
	c = exec.Command(dockerBinary, "images", "--digests")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	// make sure image 1 has repo, tag, digest
	if !reWithTag1.MatchString(out) {
		t.Fatalf("expected %q: %s", re1.String(), out)
	}

	// make sure image 2 has repo, tag, digest
	reWithTag2 := regexp.MustCompile(`\s*` + repoName + `\s*tag2\s*<none>\s`)
	reWithDigest2 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest2 + `\s`)
	if !reWithTag2.MatchString(out) {
		t.Fatalf("expected %q: %s", reWithTag2.String(), out)
	}
	if !reWithDigest2.MatchString(out) {
		t.Fatalf("expected %q: %s", reWithDigest2.String(), out)
	}

	// list images
	c = exec.Command(dockerBinary, "images", "--digests")
	out, _, err = runCommandWithOutput(c)
	if err != nil {
		t.Fatalf("error listing images: %s, %v", out, err)
	}

	// make sure image 1 has repo, tag, digest
	if !reWithTag1.MatchString(out) {
		t.Fatalf("expected %q: %s", re1.String(), out)
	}
	// make sure image 2 has repo, tag, digest
	if !reWithTag2.MatchString(out) {
		t.Fatalf("expected %q: %s", re2.String(), out)
	}
	// make sure busybox has tag, but not digest
	busyboxRe := regexp.MustCompile(`\s*busybox\s*latest\s*<none>\s`)
	if !busyboxRe.MatchString(out) {
		t.Fatalf("expected %q: %s", busyboxRe.String(), out)
	}

	logDone("by_digest - list images with digests")
}

func TestDeleteImageByIDOnlyPulledByDigest(t *testing.T) {
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
	// just in case...
	defer deleteImages(imageReference)

	imageID, err := inspectField(imageReference, ".Id")
	if err != nil {
		t.Fatalf("error inspecting image id: %v", err)
	}

	c = exec.Command(dockerBinary, "rmi", imageID)
	if _, err := runCommand(c); err != nil {
		t.Fatalf("error deleting image by id: %v", err)
	}

	logDone("by_digest - delete image by id only pulled by digest")
}
