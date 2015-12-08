package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

var (
	remoteRepoName  = "dockercli/busybox-by-dgst"
	repoName        = fmt.Sprintf("%v/%s", privateRegistryURL, remoteRepoName)
	pushDigestRegex = regexp.MustCompile("[\\S]+: digest: ([\\S]+) size: [0-9]+")
	digestRegex     = regexp.MustCompile("Digest: ([\\S]+)")
)

func setupImage(c *check.C) (digest.Digest, error) {
	return setupImageWithTag(c, "latest")
}

func setupImageWithTag(c *check.C, tag string) (digest.Digest, error) {
	containerName := "busyboxbydigest"

	dockerCmd(c, "run", "-d", "-e", "digest=1", "--name", containerName, "busybox")

	// tag the image to upload it to the private registry
	repoAndTag := repoName + ":" + tag
	out, _, err := dockerCmdWithError("commit", containerName, repoAndTag)
	c.Assert(err, checker.IsNil, check.Commentf("image tagging failed: %s", out))

	// delete the container as we don't need it any more
	err = deleteContainer(containerName)
	c.Assert(err, checker.IsNil)

	// push the image
	out, _, err = dockerCmdWithError("push", repoAndTag)
	c.Assert(err, checker.IsNil, check.Commentf("pushing the image to the private registry has failed: %s", out))

	// delete our local repo that we previously tagged
	rmiout, _, err := dockerCmdWithError("rmi", repoAndTag)
	c.Assert(err, checker.IsNil, check.Commentf("error deleting images prior to real test: %s", rmiout))

	matches := pushDigestRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from push output: %s", out))
	pushDigest := matches[1]

	return digest.Digest(pushDigest), nil
}

func (s *DockerRegistrySuite) TestPullByTagDisplaysDigest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	// pull from the registry using the tag
	out, _ := dockerCmd(c, "pull", repoName)

	// the pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	// make sure the pushed and pull digests match
	c.Assert(pushDigest.String(), checker.Equals, pullDigest)
}

func (s *DockerRegistrySuite) TestPullByDigest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	// pull from the registry using the <name>@<digest> reference
	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)
	out, _ := dockerCmd(c, "pull", imageReference)

	// the pull output includes "Digest: <digest>", so find that
	matches := digestRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from pull output: %s", out))
	pullDigest := matches[1]

	// make sure the pushed and pull digests match
	c.Assert(pushDigest.String(), checker.Equals, pullDigest)
}

func (s *DockerRegistrySuite) TestPullByDigestNoFallback(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// pull from the registry using the <name>@<digest> reference
	imageReference := fmt.Sprintf("%s@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", repoName)
	out, _, err := dockerCmdWithError("pull", imageReference)
	c.Assert(err, checker.NotNil, check.Commentf("expected non-zero exit status and correct error message when pulling non-existing image"))
	c.Assert(out, checker.Contains, "manifest unknown", check.Commentf("expected non-zero exit status and correct error message when pulling non-existing image"))
}

func (s *DockerRegistrySuite) TestCreateByDigest(c *check.C) {
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)

	containerName := "createByDigest"
	out, _ := dockerCmd(c, "create", "--name", containerName, imageReference)

	res, err := inspectField(containerName, "Config.Image")
	c.Assert(err, checker.IsNil, check.Commentf("failed to get Config.Image: %s", out))
	c.Assert(res, checker.Equals, imageReference)
}

func (s *DockerRegistrySuite) TestRunByDigest(c *check.C) {
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil)

	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)

	containerName := "runByDigest"
	out, _ := dockerCmd(c, "run", "--name", containerName, imageReference, "sh", "-c", "echo found=$digest")

	foundRegex := regexp.MustCompile("found=([^\n]+)")
	matches := foundRegex.FindStringSubmatch(out)
	c.Assert(matches, checker.HasLen, 2, check.Commentf("unable to parse digest from pull output: %s", out))
	c.Assert(matches[1], checker.Equals, "1", check.Commentf("Expected %q, got %q", "1", matches[1]))

	res, err := inspectField(containerName, "Config.Image")
	c.Assert(err, checker.IsNil, check.Commentf("failed to get Config.Image: %s", out))
	c.Assert(res, checker.Equals, imageReference)
}

func (s *DockerRegistrySuite) TestRemoveImageByDigest(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	// make sure inspect runs ok
	_, err = inspectField(imageReference, "Id")
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect image"))

	// do the delete
	err = deleteImages(imageReference)
	c.Assert(err, checker.IsNil, check.Commentf("unexpected error deleting image"))

	// try to inspect again - it should error this time
	_, err = inspectField(imageReference, "Id")
	//unexpected nil err trying to inspect what should be a non-existent image
	c.Assert(err, checker.NotNil)
	c.Assert(err.Error(), checker.Contains, "No such image")
}

func (s *DockerRegistrySuite) TestBuildByDigest(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	// get the image id
	imageID, err := inspectField(imageReference, "Id")
	c.Assert(err, checker.IsNil, check.Commentf("error getting image id"))

	// do the build
	name := "buildbydigest"
	_, err = buildImage(name, fmt.Sprintf(
		`FROM %s
     CMD ["/bin/echo", "Hello World"]`, imageReference),
		true)
	c.Assert(err, checker.IsNil)

	// get the build's image id
	res, err := inspectField(name, "Config.Image")
	c.Assert(err, checker.IsNil)
	// make sure they match
	c.Assert(res, checker.Equals, imageID)
}

func (s *DockerRegistrySuite) TestTagByDigest(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	// tag it
	tag := "tagbydigest"
	dockerCmd(c, "tag", imageReference, tag)

	expectedID, err := inspectField(imageReference, "Id")
	c.Assert(err, checker.IsNil, check.Commentf("error getting original image id"))

	tagID, err := inspectField(tag, "Id")
	c.Assert(err, checker.IsNil, check.Commentf("error getting tagged image id"))
	c.Assert(tagID, checker.Equals, expectedID)
}

func (s *DockerRegistrySuite) TestListImagesWithoutDigests(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	out, _ := dockerCmd(c, "images")
	c.Assert(out, checker.Not(checker.Contains), "DIGEST", check.Commentf("list output should not have contained DIGEST header"))
}

func (s *DockerRegistrySuite) TestListImagesWithDigests(c *check.C) {

	// setup image1
	digest1, err := setupImageWithTag(c, "tag1")
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))
	imageReference1 := fmt.Sprintf("%s@%s", repoName, digest1)
	c.Logf("imageReference1 = %s", imageReference1)

	// pull image1 by digest
	dockerCmd(c, "pull", imageReference1)

	// list images
	out, _ := dockerCmd(c, "images", "--digests")

	// make sure repo shown, tag=<none>, digest = $digest1
	re1 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest1.String() + `\s`)
	c.Assert(re1.MatchString(out), checker.True, check.Commentf("expected %q: %s", re1.String(), out))
	// setup image2
	digest2, err := setupImageWithTag(c, "tag2")
	//error setting up image
	c.Assert(err, checker.IsNil)
	imageReference2 := fmt.Sprintf("%s@%s", repoName, digest2)
	c.Logf("imageReference2 = %s", imageReference2)

	// pull image1 by digest
	dockerCmd(c, "pull", imageReference1)

	// pull image2 by digest
	dockerCmd(c, "pull", imageReference2)

	// list images
	out, _ = dockerCmd(c, "images", "--digests")

	// make sure repo shown, tag=<none>, digest = $digest1
	c.Assert(re1.MatchString(out), checker.True, check.Commentf("expected %q: %s", re1.String(), out))

	// make sure repo shown, tag=<none>, digest = $digest2
	re2 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest2.String() + `\s`)
	c.Assert(re2.MatchString(out), checker.True, check.Commentf("expected %q: %s", re2.String(), out))

	// pull tag1
	dockerCmd(c, "pull", repoName+":tag1")

	// list images
	out, _ = dockerCmd(c, "images", "--digests")

	// make sure image 1 has repo, tag, <none> AND repo, <none>, digest
	reWithTag1 := regexp.MustCompile(`\s*` + repoName + `\s*tag1\s*<none>\s`)
	reWithDigest1 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest1.String() + `\s`)
	c.Assert(reWithDigest1.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithDigest1.String(), out))
	c.Assert(reWithTag1.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithTag1.String(), out))
	// make sure image 2 has repo, <none>, digest
	c.Assert(re2.MatchString(out), checker.True, check.Commentf("expected %q: %s", re2.String(), out))

	// pull tag 2
	dockerCmd(c, "pull", repoName+":tag2")

	// list images
	out, _ = dockerCmd(c, "images", "--digests")

	// make sure image 1 has repo, tag, digest
	c.Assert(reWithTag1.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithTag1.String(), out))

	// make sure image 2 has repo, tag, digest
	reWithTag2 := regexp.MustCompile(`\s*` + repoName + `\s*tag2\s*<none>\s`)
	reWithDigest2 := regexp.MustCompile(`\s*` + repoName + `\s*<none>\s*` + digest2.String() + `\s`)
	c.Assert(reWithTag2.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithTag2.String(), out))
	c.Assert(reWithDigest2.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithDigest2.String(), out))

	// list images
	out, _ = dockerCmd(c, "images", "--digests")

	// make sure image 1 has repo, tag, digest
	c.Assert(reWithTag1.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithTag1.String(), out))
	// make sure image 2 has repo, tag, digest
	c.Assert(reWithTag2.MatchString(out), checker.True, check.Commentf("expected %q: %s", reWithTag2.String(), out))
	// make sure busybox has tag, but not digest
	busyboxRe := regexp.MustCompile(`\s*busybox\s*latest\s*<none>\s`)
	c.Assert(busyboxRe.MatchString(out), checker.True, check.Commentf("expected %q: %s", busyboxRe.String(), out))
}

func (s *DockerRegistrySuite) TestInspectImageWithDigests(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, check.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	out, _ := dockerCmd(c, "inspect", imageReference)

	var imageJSON []types.ImageInspect
	err = json.Unmarshal([]byte(out), &imageJSON)
	c.Assert(err, checker.IsNil)
	c.Assert(imageJSON, checker.HasLen, 1)
	c.Assert(imageJSON[0].RepoDigests, checker.HasLen, 1)
	c.Assert(stringutils.InSlice(imageJSON[0].RepoDigests, imageReference), checker.Equals, true)
}

func (s *DockerRegistrySuite) TestPsListContainersFilterAncestorImageByDigest(c *check.C) {
	digest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	imageReference := fmt.Sprintf("%s@%s", repoName, digest)

	// pull from the registry using the <name>@<digest> reference
	dockerCmd(c, "pull", imageReference)

	// build a image from it
	imageName1 := "images_ps_filter_test"
	_, err = buildImage(imageName1, fmt.Sprintf(
		`FROM %s
		 LABEL match me 1`, imageReference), true)
	c.Assert(err, checker.IsNil)

	// run a container based on that
	out, _ := dockerCmd(c, "run", "-d", imageReference, "echo", "hello")
	expectedID := strings.TrimSpace(out)

	// run a container based on the a descendant of that too
	out, _ = dockerCmd(c, "run", "-d", imageName1, "echo", "hello")
	expectedID1 := strings.TrimSpace(out)

	expectedIDs := []string{expectedID, expectedID1}

	// Invalid imageReference
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", fmt.Sprintf("--filter=ancestor=busybox@%s", digest))
	// Filter container for ancestor filter should be empty
	c.Assert(strings.TrimSpace(out), checker.Equals, "")

	// Valid imageReference
	out, _ = dockerCmd(c, "ps", "-a", "-q", "--no-trunc", "--filter=ancestor="+imageReference)
	checkPsAncestorFilterOutput(c, out, imageReference, expectedIDs)
}

func (s *DockerRegistrySuite) TestDeleteImageByIDOnlyPulledByDigest(c *check.C) {
	pushDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	// pull from the registry using the <name>@<digest> reference
	imageReference := fmt.Sprintf("%s@%s", repoName, pushDigest)
	dockerCmd(c, "pull", imageReference)
	// just in case...

	imageID, err := inspectField(imageReference, "Id")
	c.Assert(err, checker.IsNil, check.Commentf("error inspecting image id"))

	dockerCmd(c, "rmi", imageID)
}

// TestPullFailsWithAlteredManifest tests that a `docker pull` fails when
// we have modified a manifest blob and its digest cannot be verified.
func (s *DockerRegistrySuite) TestPullFailsWithAlteredManifest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	manifestDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil, check.Commentf("error setting up image"))

	// Load the target manifest blob.
	manifestBlob := s.reg.readBlobContents(c, manifestDigest)

	var imgManifest schema1.Manifest
	err = json.Unmarshal(manifestBlob, &imgManifest)
	c.Assert(err, checker.IsNil, check.Commentf("unable to decode image manifest from blob"))

	// Change a layer in the manifest.
	imgManifest.FSLayers[0] = schema1.FSLayer{
		BlobSum: digest.Digest("sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
	}

	// Move the existing data file aside, so that we can replace it with a
	// malicious blob of data. NOTE: we defer the returned undo func.
	undo := s.reg.tempMoveBlobData(c, manifestDigest)
	defer undo()

	alteredManifestBlob, err := json.MarshalIndent(imgManifest, "", "   ")
	c.Assert(err, checker.IsNil, check.Commentf("unable to encode altered image manifest to JSON"))

	s.reg.writeBlobContents(c, manifestDigest, alteredManifestBlob)

	// Now try pulling that image by digest. We should get an error about
	// digest verification for the manifest digest.

	// Pull from the registry using the <name>@<digest> reference.
	imageReference := fmt.Sprintf("%s@%s", repoName, manifestDigest)
	out, exitStatus, _ := dockerCmdWithError("pull", imageReference)
	c.Assert(exitStatus, checker.Not(check.Equals), 0)

	expectedErrorMsg := fmt.Sprintf("image verification failed for digest %s", manifestDigest)
	c.Assert(out, checker.Contains, expectedErrorMsg)
}

// TestPullFailsWithAlteredLayer tests that a `docker pull` fails when
// we have modified a layer blob and its digest cannot be verified.
func (s *DockerRegistrySuite) TestPullFailsWithAlteredLayer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	manifestDigest, err := setupImage(c)
	c.Assert(err, checker.IsNil)

	// Load the target manifest blob.
	manifestBlob := s.reg.readBlobContents(c, manifestDigest)

	var imgManifest schema1.Manifest
	err = json.Unmarshal(manifestBlob, &imgManifest)
	c.Assert(err, checker.IsNil)

	// Next, get the digest of one of the layers from the manifest.
	targetLayerDigest := imgManifest.FSLayers[0].BlobSum

	// Move the existing data file aside, so that we can replace it with a
	// malicious blob of data. NOTE: we defer the returned undo func.
	undo := s.reg.tempMoveBlobData(c, targetLayerDigest)
	defer undo()

	// Now make a fake data blob in this directory.
	s.reg.writeBlobContents(c, targetLayerDigest, []byte("This is not the data you are looking for."))

	// Now try pulling that image by digest. We should get an error about
	// digest verification for the target layer digest.

	// Remove distribution cache to force a re-pull of the blobs
	if err := os.RemoveAll(filepath.Join(dockerBasePath, "image", s.d.storageDriver, "distribution")); err != nil {
		c.Fatalf("error clearing distribution cache: %v", err)
	}

	// Pull from the registry using the <name>@<digest> reference.
	imageReference := fmt.Sprintf("%s@%s", repoName, manifestDigest)
	out, exitStatus, _ := dockerCmdWithError("pull", imageReference)
	c.Assert(exitStatus, checker.Not(check.Equals), 0, check.Commentf("expected a zero exit status"))

	expectedErrorMsg := fmt.Sprintf("filesystem layer verification failed for digest %s", targetLayerDigest)
	c.Assert(out, checker.Contains, expectedErrorMsg, check.Commentf("expected error message in output: %s", out))
}
